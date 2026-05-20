package commands

import (
	"slices"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// refGroup is the parameterized helper shared by `ezida columns` and
// `ezida priorities`. Each subcommand instantiates a refGroup whose
// hooks point at the right list (`[board].columns` vs
// `[board].priorities`) and the right card-field accessor.
//
// Methods operate purely on an in-memory *board.Board passed in by the
// caller; refGroup never loads or saves. The mutate-and-save dance is
// the caller's responsibility (mutateAndSave from mutate.go).
type refGroup struct {
	// list points at the slice to mutate.
	list *[]string
	// cardField returns a pointer to the relevant string field of a card
	// (`&c.Column` for columns, `&c.Priority` for priorities).
	cardField func(*board.Card) *string
	// isReferencing reports whether a card's field value (first arg)
	// references the target name (second arg). For columns the
	// definition is value == name. For priorities the definition is
	// value != "" && value == name (cards without a priority do NOT
	// count).
	isReferencing func(value, name string) bool
	// inUseErr builds the "still referenced" error.
	inUseErr func(name string, cards []affectedCard) error
	// lastErr builds the "would leave the list empty" error.
	lastErr func(name string) error
	// duplicateErr builds the "already exists" error.
	duplicateErr func(name string) error
	// unknownErr builds the "no such entry" error (e.g. COLUMN_NOT_FOUND
	// or INVALID_PRIORITY).
	unknownErr func(name string) error
	// positionErr builds the POSITION_OUT_OF_RANGE error. Used by add.
	positionErr func(position, max int) error
}

// add inserts name into *g.list. position is 1-indexed; 0 means
// "append at the end" (the columns default). Out-of-range values trigger
// positionErr. Duplicate names trigger duplicateErr.
func (g *refGroup) add(name string, position int) error {
	for _, existing := range *g.list {
		if existing == name {
			return g.duplicateErr(name)
		}
	}
	if position == 0 {
		*g.list = append(*g.list, name)
		return nil
	}
	maxPos := len(*g.list) + 1
	if position < 1 || position > maxPos {
		return g.positionErr(position, maxPos)
	}
	// slices.Insert is 0-indexed; convert.
	*g.list = slices.Insert(*g.list, position-1, name)
	return nil
}

// rename swaps oldName for newName in *g.list and rewrites every card
// whose accessor field referenced oldName so it references newName.
// Renaming to the same name is a no-op.
func (g *refGroup) rename(b *board.Board, oldName, newName string) error {
	if oldName == newName {
		return nil
	}
	idx := -1
	for i, n := range *g.list {
		if n == oldName {
			idx = i
		}
	}
	if idx < 0 {
		return g.unknownErr(oldName)
	}
	for _, existing := range *g.list {
		if existing == newName {
			return g.duplicateErr(newName)
		}
	}
	(*g.list)[idx] = newName
	for i := range b.Cards {
		ptr := g.cardField(&b.Cards[i])
		if g.isReferencing(*ptr, oldName) {
			*ptr = newName
		}
	}
	return nil
}

// remove deletes name from *g.list. It collects every card that
// references name first; if any exist, returns inUseErr without
// modifying the board. If the list would become empty, returns lastErr.
// If name is not present, returns unknownErr.
func (g *refGroup) remove(b *board.Board, name string) error {
	idx := -1
	for i, n := range *g.list {
		if n == name {
			idx = i
		}
	}
	if idx < 0 {
		return g.unknownErr(name)
	}
	var violators []affectedCard
	for _, c := range b.Cards {
		ptr := g.cardField(&c)
		if g.isReferencing(*ptr, name) {
			violators = append(violators, affectedCard{ID: c.ID, Title: c.Title})
		}
	}
	if len(violators) > 0 {
		return g.inUseErr(name, violators)
	}
	if len(*g.list) == 1 {
		return g.lastErr(name)
	}
	*g.list = slices.Delete(*g.list, idx, idx+1)
	return nil
}
