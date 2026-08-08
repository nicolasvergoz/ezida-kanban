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

// EpicProgress counts the children of id, and how many of them sit in a
// terminal column. A board with no terminal column truthfully reports
// done = 0; that is a reading of the board's configuration, not an
// error worth warning about.
func EpicProgress(b *Board, id string) (done, total int) {
	for _, c := range ChildrenOf(b, id) {
		total++
		if b.Board.IsDoneColumn(c.Column) {
			done++
		}
	}
	return done, total
}

// IsEpic reports whether id is referenced as the epic of at least one
// card. A card carrying a color but no children is not an epic.
func IsEpic(b *Board, id string) bool {
	if id == "" {
		return false
	}
	for _, c := range b.Cards {
		if c.Epic == id {
			return true
		}
	}
	return false
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
// the three pre-mutation rules in the order the CLI reports them:
// unknown target, self-reference, and target-already-a-child.
//
// Returns *InvalidEpicError on any violation, leaving the caller free
// to reject before mutating anything.
func CheckEpicTarget(b *Board, childID, epicID string) error {
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
