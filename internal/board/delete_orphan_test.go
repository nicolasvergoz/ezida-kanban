package board

import (
	"reflect"
	"testing"
	"time"
)

// orphanBoard is a parent with a color, two children, and one
// unrelated card.
func orphanBoard() *Board {
	now := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	return &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    []string{"todo", "done"},
			Priorities: []string{"low"},
		},
		Cards: []Card{
			{ID: "rl4m9x", Title: "parent", Column: "todo", Tags: []string{},
				Color: "#8b5cf6", CreatedAt: now, UpdatedAt: now},
			{ID: "f20wbo", Title: "child one", Column: "todo", Tags: []string{},
				Epic: "rl4m9x", CreatedAt: now, UpdatedAt: now},
			{ID: "a3f2k9", Title: "loose", Column: "done", Tags: []string{},
				CreatedAt: now, UpdatedAt: now},
			{ID: "wrshlo", Title: "child two", Column: "done", Tags: []string{},
				Epic: "rl4m9x", CreatedAt: now, UpdatedAt: now},
		},
	}
}

// Deleting a child must leave the parent completely alone — including
// its color and its timestamp.
func TestDeleteCardOrphaning_DeletingAChildTouchesNothingElse(t *testing.T) {
	b := orphanBoard()
	before := b.Cards[0]

	orphaned, err := DeleteCardOrphaning(b, "f20wbo")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(orphaned) != 0 {
		t.Fatalf("orphaned = %v, want none", orphaned)
	}
	if !reflect.DeepEqual(b.Cards[0], before) {
		t.Fatalf("parent changed: %+v, want %+v", b.Cards[0], before)
	}
	// The surviving child keeps its link.
	if b.Cards[2].Epic != "rl4m9x" {
		t.Errorf("sibling lost its epic: %q", b.Cards[2].Epic)
	}
}

// Deleting a parent clears exactly the referencing cards, in board file
// order, and refreshes nobody's timestamp: losing a parent is a
// board-level consequence, not an edit to the child.
func TestDeleteCardOrphaning_DeletingAParentClearsItsChildren(t *testing.T) {
	b := orphanBoard()
	loose := b.Cards[2]
	childStamps := map[string]time.Time{
		"f20wbo": b.Cards[1].UpdatedAt,
		"wrshlo": b.Cards[3].UpdatedAt,
	}

	orphaned, err := DeleteCardOrphaning(b, "rl4m9x")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	want := []string{"f20wbo", "wrshlo"}
	if len(orphaned) != len(want) {
		t.Fatalf("orphaned = %v, want %v", orphaned, want)
	}
	for i := range want {
		if orphaned[i] != want[i] {
			t.Fatalf("orphaned = %v, want %v (file order)", orphaned, want)
		}
	}
	for _, c := range b.Cards {
		if c.Epic != "" {
			t.Errorf("card %s still references a deleted epic", c.ID)
		}
		if stamp, ok := childStamps[c.ID]; ok && !c.UpdatedAt.Equal(stamp) {
			t.Errorf("card %s had its updated_at refreshed by orphaning", c.ID)
		}
	}
	// The unrelated card is untouched, field for field.
	for _, c := range b.Cards {
		if c.ID == "a3f2k9" && !reflect.DeepEqual(c, loose) {
			t.Errorf("unrelated card changed: %+v, want %+v", c, loose)
		}
	}
	// The board is still valid after the orphaning.
	if verr := Validate(b); verr != nil {
		t.Fatalf("board invalid after orphaning: %v", verr)
	}
}

func TestDeleteCardOrphaning_UnknownIDLeavesBoardIntact(t *testing.T) {
	b := orphanBoard()
	before := len(b.Cards)
	if _, err := DeleteCardOrphaning(b, "zzzzzz"); err == nil {
		t.Fatal("expected *CardNotFoundError")
	}
	if len(b.Cards) != before {
		t.Fatalf("cards = %d, want %d", len(b.Cards), before)
	}
}
