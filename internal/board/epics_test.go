package board

import (
	"testing"
	"time"
)

// epicBoard builds a board with one parent (`parent`) and three
// children declared in the file order [c, a, b], so tests can assert
// that derived children follow file order rather than id order.
func epicBoard(t *testing.T, terminal string) *Board {
	t.Helper()
	now := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	card := func(id, column, epic string) Card {
		return Card{
			ID: id, Title: "card " + id, Column: column, Tags: []string{},
			Epic: epic, CreatedAt: now, UpdatedAt: now,
		}
	}
	b := &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    []string{"todo", "done"},
			Priorities: []string{"low"},
		},
		Cards: []Card{
			card("rl4m9x", "todo", ""),
			card("ccccc1", "done", "rl4m9x"),
			card("aaaaa1", "todo", "rl4m9x"),
			card("bbbbb1", "todo", "rl4m9x"),
			card("loose1", "todo", ""),
		},
	}
	if terminal != "" {
		b.Board.SetDoneColumn(terminal, true)
	}
	return b
}

func TestChildrenOf_PreservesFileOrder(t *testing.T) {
	b := epicBoard(t, "done")
	children := ChildrenOf(b, "rl4m9x")
	want := []string{"ccccc1", "aaaaa1", "bbbbb1"}
	if len(children) != len(want) {
		t.Fatalf("got %d children, want %d", len(children), len(want))
	}
	for i, c := range children {
		if c.ID != want[i] {
			t.Fatalf("children = %v, want %v", children, want)
		}
	}
	if got := ChildrenOf(b, "loose1"); got != nil {
		t.Errorf("ChildrenOf(childless) = %v, want nil", got)
	}
	if got := ChildrenOf(b, ""); got != nil {
		t.Errorf("ChildrenOf(\"\") = %v, want nil", got)
	}
}

func TestEpicProgress_CountsTerminalColumns(t *testing.T) {
	b := epicBoard(t, "done")
	done, total := EpicProgress(b, "rl4m9x")
	if done != 1 || total != 3 {
		t.Fatalf("progress = %d/%d, want 1/3", done, total)
	}
}

// A board that declares no terminal column truthfully reports zero
// progress; that is a reading of the configuration, not an error.
func TestEpicProgress_NoTerminalColumnYieldsZero(t *testing.T) {
	b := epicBoard(t, "")
	done, total := EpicProgress(b, "rl4m9x")
	if done != 0 || total != 3 {
		t.Fatalf("progress = %d/%d, want 0/3", done, total)
	}
}

func TestIsEpicAndParentOf(t *testing.T) {
	b := epicBoard(t, "done")
	if !IsEpic(b, "rl4m9x") {
		t.Error("parent not reported as an epic")
	}
	if IsEpic(b, "loose1") {
		t.Error("childless card reported as an epic")
	}
	if IsEpic(b, "") {
		t.Error("empty id reported as an epic")
	}
	parent := ParentOf(b, "aaaaa1")
	if parent == nil || parent.ID != "rl4m9x" {
		t.Fatalf("ParentOf(child) = %v, want rl4m9x", parent)
	}
	if got := ParentOf(b, "loose1"); got != nil {
		t.Errorf("ParentOf(orphan) = %v, want nil", got)
	}
}

func TestCheckEpicTarget(t *testing.T) {
	b := epicBoard(t, "done")
	if err := CheckEpicTarget(b, "loose1", "rl4m9x"); err != nil {
		t.Fatalf("legal target rejected: %v", err)
	}
	if err := CheckEpicTarget(b, "loose1", ""); err != nil {
		t.Fatalf("empty target rejected: %v", err)
	}
	if err := CheckEpicTarget(b, "loose1", "loose1"); err == nil {
		t.Error("self-reference accepted")
	}
	if err := CheckEpicTarget(b, "loose1", "zzzzzz"); err == nil {
		t.Error("unknown target accepted")
	}
	// aaaaa1 is itself a child, so it may not become a parent.
	err := CheckEpicTarget(b, "loose1", "aaaaa1")
	if err == nil {
		t.Fatal("nested target accepted")
	}
	var ie *InvalidEpicError
	if !asInvalidEpic(err, &ie) {
		t.Fatalf("error = %T, want *InvalidEpicError", err)
	}
	// The refusal must explain itself: the target looks like an
	// ordinary card, so "id rejected" alone would be unactionable.
	if ie.Reason == "" {
		t.Error("nesting refusal carried no reason")
	}
}

func TestEnsureEpicColor(t *testing.T) {
	b := epicBoard(t, "done")
	if !EnsureEpicColor(b, "rl4m9x") {
		t.Fatal("EnsureEpicColor reported no assignment on an uncolored card")
	}
	if b.Cards[0].Color != "#8b5cf6" {
		t.Fatalf("assigned color = %q, want #8b5cf6", b.Cards[0].Color)
	}
	// An explicit color always survives.
	b.Cards[0].Color = "#7c3aed"
	if EnsureEpicColor(b, "rl4m9x") {
		t.Error("EnsureEpicColor overwrote an existing color")
	}
	if b.Cards[0].Color != "#7c3aed" {
		t.Fatalf("color = %q, want #7c3aed", b.Cards[0].Color)
	}
	if EnsureEpicColor(b, "zzzzzz") {
		t.Error("EnsureEpicColor reported an assignment for an unknown card")
	}
}

// asInvalidEpic keeps the errors.As import out of the table above.
func asInvalidEpic(err error, target **InvalidEpicError) bool {
	ie, ok := err.(*InvalidEpicError)
	if ok {
		*target = ie
	}
	return ok
}
