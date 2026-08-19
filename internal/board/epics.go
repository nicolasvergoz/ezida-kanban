package board

import "fmt"

// InvalidEpicError is returned when a caller attempts to point a card
// at an epic that does not exist, at itself, or at a card that already
// belongs to an epic. Reason carries the human sentence — for the
// nesting case it must explain *why* the id was refused, since the
// target looks like a perfectly ordinary card to the user.
//
// The CLI maps this to the wire code INVALID_EPIC.
type InvalidEpicError struct {
	ID     string
	Reason string
}

func (e *InvalidEpicError) Error() string {
	return fmt.Sprintf("board: epic %q is invalid: %s", e.ID, e.Reason)
}

// ChildrenOf returns every card whose Epic equals id, in board file
// order. File order is the only order an epic's children have (design
// non-goal: no per-epic reordering).
func ChildrenOf(b *Board, id string) []Card {
	if id == "" {
		return nil
	}
	var out []Card
	for _, c := range b.Cards {
		if c.Epic == id {
			out = append(out, c)
		}
	}
	return out
}

// ArchivedChildrenOf returns every archived card whose Epic equals id,
// in archive file order. A nil archive yields nothing — every caller
// can pass the result of loadArchive without a nil check of its own.
func ArchivedChildrenOf(a *Archive, id string) []ArchivedCard {
	if id == "" || a == nil {
		return nil
	}
	var out []ArchivedCard
	for _, c := range a.Cards {
		if c.Epic == id {
			out = append(out, c)
		}
	}
	return out
}

// EpicProgress counts the children of id — live and archived — and how
// many of them sit in a terminal column. An archived child is judged
// by the column the archive recorded for it, checked against the
// board's terminal columns at read time: the same rule a live card
// follows. A board with no terminal column truthfully reports done =
// 0; that is a reading of the board's configuration, not an error
// worth warning about. A nil archive reproduces the live-only
// behaviour this function had before archiving existed.
func EpicProgress(b *Board, a *Archive, id string) (done, total int) {
	for _, c := range ChildrenOf(b, id) {
		total++
		if b.Board.IsDoneColumn(c.Column) {
			done++
		}
	}
	for _, c := range ArchivedChildrenOf(a, id) {
		total++
		if b.Board.IsDoneColumn(c.Column) {
			done++
		}
	}
	return done, total
}

// IsEpic reports whether id is referenced as the epic of at least one
// card, live or archived. A card carrying a color but no children is
// not an epic. A nil archive reduces this to the live-only check.
func IsEpic(b *Board, a *Archive, id string) bool {
	if id == "" {
		return false
	}
	for _, c := range b.Cards {
		if c.Epic == id {
			return true
		}
	}
	return len(ArchivedChildrenOf(a, id)) > 0
}

// ParentOf returns the card named by the Epic field of the card with
// the given id, or nil when the card carries no epic (or neither card
// exists). The returned pointer aliases b.Cards.
func ParentOf(b *Board, id string) *Card {
	var epic string
	for _, c := range b.Cards {
		if c.ID == id {
			epic = c.Epic
			break
		}
	}
	if epic == "" {
		return nil
	}
	for i := range b.Cards {
		if b.Cards[i].ID == epic {
			return &b.Cards[i]
		}
	}
	return nil
}

// CheckEpicTarget validates that childID may point at epicID, applying
// the four pre-mutation rules in the order the CLI reports them:
// self-reference, unknown target, target-already-a-child, and
// child-already-a-parent.
//
// The last rule is the mirror of the third: giving an epic a parent of
// its own pushes its existing children to a second level. Without it
// the case is caught only afterwards by the whole-board Validate,
// which reports a board-level inconsistency for what is a single
// invalid argument.
//
// Returns *InvalidEpicError on any violation, leaving the caller free
// to reject before mutating anything.
func CheckEpicTarget(b *Board, a *Archive, childID, epicID string) error {
	if epicID == "" {
		return nil
	}
	if epicID == childID {
		return &InvalidEpicError{ID: epicID, Reason: "a card cannot be its own epic"}
	}
	var target *Card
	for i := range b.Cards {
		if b.Cards[i].ID == epicID {
			target = &b.Cards[i]
			break
		}
	}
	if target == nil {
		return &InvalidEpicError{ID: epicID, Reason: "no card on this board carries that id"}
	}
	if target.Epic != "" {
		return &InvalidEpicError{
			ID: epicID,
			Reason: fmt.Sprintf(
				"that card already belongs to epic %q, and epic nesting is limited to one level",
				target.Epic),
		}
	}
	if IsEpic(b, a, childID) {
		return &InvalidEpicError{
			ID:     epicID,
			Reason: "the card being edited has children of its own, and epic nesting is limited to one level",
		}
	}
	return nil
}

// EnsureEpicColor gives the card with the given id a palette color when
// it has none, and reports whether it wrote one. Acquiring children is
// what makes a card an epic, and an epic without a color has nothing to
// lend its children in presentation surfaces.
//
// An explicit color always survives.
func EnsureEpicColor(b *Board, id string) bool {
	for i := range b.Cards {
		if b.Cards[i].ID != id {
			continue
		}
		if b.Cards[i].Color != "" {
			return false
		}
		b.Cards[i].Color = AssignColor(b)
		return true
	}
	return false
}
