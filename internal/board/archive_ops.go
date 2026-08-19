package board

import "time"

// ArchiveCard removes id and its epic children (one level, per the
// nesting cap) from b and prepends them to a, all sharing at as their
// ArchivedAt timestamp. The parent is placed before its children, and
// the children keep the relative order they held in b.Cards. Neither
// UpdatedAt nor any child's Epic field is modified — archiving is not
// a content edit. Pure: never loads or saves either file.
func ArchiveCard(b *Board, a *Archive, id string, at time.Time) ([]ArchivedCard, error) {
	idx := -1
	for i, c := range b.Cards {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, &CardNotFoundError{ID: id}
	}
	parent := b.Cards[idx]
	children := ChildrenOf(b, id)

	toArchive := make([]Card, 0, 1+len(children))
	toArchive = append(toArchive, parent)
	toArchive = append(toArchive, children...)

	archived := archiveSlice(toArchive, at)
	removeFromBoard(b, toArchive)
	prependToArchive(a, archived)
	return archived, nil
}

// ArchiveColumn archives every card whose Column matches column,
// together with the epic children of those cards wherever those
// children live. direct and cascaded are disjoint by construction; an
// unknown column returns *ColumnNotFoundError. An empty column returns
// two empty, non-nil slices and does not mutate b or a. Pure: never
// loads or saves either file.
func ArchiveColumn(b *Board, a *Archive, column string, at time.Time) (direct, cascaded []ArchivedCard, err error) {
	found := false
	for _, col := range b.Board.Columns {
		if col == column {
			found = true
			break
		}
	}
	if !found {
		return nil, nil, &ColumnNotFoundError{Column: column}
	}

	directIDs := make(map[string]struct{})
	for _, c := range b.Cards {
		if c.Column == column {
			directIDs[c.ID] = struct{}{}
		}
	}
	if len(directIDs) == 0 {
		return []ArchivedCard{}, []ArchivedCard{}, nil
	}

	cascadedIDs := make(map[string]struct{})
	for _, c := range b.Cards {
		if c.Epic == "" {
			continue
		}
		if _, parentIsDirect := directIDs[c.Epic]; !parentIsDirect {
			continue
		}
		if _, alreadyDirect := directIDs[c.ID]; alreadyDirect {
			continue
		}
		cascadedIDs[c.ID] = struct{}{}
	}

	var directList, cascadedList []Card
	for _, c := range b.Cards {
		if _, ok := directIDs[c.ID]; ok {
			directList = append(directList, c)
			continue
		}
		if _, ok := cascadedIDs[c.ID]; ok {
			cascadedList = append(cascadedList, c)
		}
	}

	direct = archiveSlice(directList, at)
	cascaded = archiveSlice(cascadedList, at)

	removeFromBoard(b, append(append([]Card{}, directList...), cascadedList...))
	prependToArchive(a, append(append([]ArchivedCard{}, direct...), cascaded...))

	return direct, cascaded, nil
}

// UnarchiveCard restores id and its archived children (archived cards
// naming id as their Epic) to b in one all-or-nothing operation. Each
// restored card returns to the column named by its stored Column when
// that column still exists on b; otherwise it lands in b's first
// column and relocated is reported true. An explicit column, when
// non-empty, overrides every restored card's destination and must
// already exist on b. A restored card whose Epic is neither live nor
// part of this same restore has its Epic cleared and is reported in
// orphaned. Restoring an id already live on b is refused with
// *IDCollisionError before any mutation. Pure: never loads or saves
// either file.
func UnarchiveCard(b *Board, a *Archive, id, column string) (restored []Card, orphaned []string, relocated bool, err error) {
	idx := -1
	for i, c := range a.Cards {
		if c.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, nil, false, &CardNotArchivedError{ID: id}
	}

	if column != "" {
		found := false
		for _, col := range b.Board.Columns {
			if col == column {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, false, &ColumnNotFoundError{Column: column}
		}
	}

	parent := a.Cards[idx]
	var children []ArchivedCard
	for _, c := range a.Cards {
		if c.Epic == id {
			children = append(children, c)
		}
	}
	toRestore := make([]ArchivedCard, 0, 1+len(children))
	toRestore = append(toRestore, parent)
	toRestore = append(toRestore, children...)

	liveIDs := make(map[string]struct{}, len(b.Cards))
	for _, c := range b.Cards {
		liveIDs[c.ID] = struct{}{}
	}
	for _, c := range toRestore {
		if _, exists := liveIDs[c.ID]; exists {
			return nil, nil, false, &IDCollisionError{ID: c.ID}
		}
	}

	colSet := make(map[string]struct{}, len(b.Board.Columns))
	for _, col := range b.Board.Columns {
		colSet[col] = struct{}{}
	}
	firstColumn := ""
	if len(b.Board.Columns) > 0 {
		firstColumn = b.Board.Columns[0]
	}

	restoredIDs := make(map[string]struct{}, len(toRestore))
	for _, c := range toRestore {
		restoredIDs[c.ID] = struct{}{}
	}

	orphanedSet := []string{}
	finalCards := make([]Card, len(toRestore))
	for i, ac := range toRestore {
		c := ac.Card
		switch {
		case column != "":
			c.Column = column
		default:
			if _, ok := colSet[c.Column]; !ok {
				c.Column = firstColumn
				relocated = true
			}
		}
		if c.Epic != "" {
			_, live := liveIDs[c.Epic]
			_, sameRestore := restoredIDs[c.Epic]
			if !live && !sameRestore {
				c.Epic = ""
				orphanedSet = append(orphanedSet, c.ID)
			}
		}
		finalCards[i] = c
	}

	keptArchive := make([]ArchivedCard, 0, len(a.Cards)-len(toRestore))
	for _, c := range a.Cards {
		if _, out := restoredIDs[c.ID]; out {
			continue
		}
		keptArchive = append(keptArchive, c)
	}
	a.Cards = keptArchive

	// Insert in reverse toRestore order: PrependCardToColumn always
	// places its argument at the top of its destination column, so
	// inserting last-to-first reproduces toRestore's own order
	// (parent, then children) as the final top-to-bottom board order.
	for i := len(finalCards) - 1; i >= 0; i-- {
		PrependCardToColumn(b, finalCards[i])
	}

	return finalCards, orphanedSet, relocated, nil
}

// archiveSlice wraps each Card in cards as an ArchivedCard sharing at
// as its ArchivedAt.
func archiveSlice(cards []Card, at time.Time) []ArchivedCard {
	out := make([]ArchivedCard, len(cards))
	for i, c := range cards {
		out[i] = ArchivedCard{Card: c, ArchivedAt: at}
	}
	return out
}

// removeFromBoard deletes every card in remove from b.Cards, preserving
// the relative order of the survivors.
func removeFromBoard(b *Board, remove []Card) {
	drop := make(map[string]struct{}, len(remove))
	for _, c := range remove {
		drop[c.ID] = struct{}{}
	}
	kept := make([]Card, 0, len(b.Cards)-len(remove))
	for _, c := range b.Cards {
		if _, out := drop[c.ID]; out {
			continue
		}
		kept = append(kept, c)
	}
	b.Cards = kept
}

// prependToArchive inserts cards at the head of a.Cards, ahead of
// everything already archived.
func prependToArchive(a *Archive, cards []ArchivedCard) {
	if len(cards) == 0 {
		return
	}
	a.Cards = append(append([]ArchivedCard{}, cards...), a.Cards...)
}
