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
	done, total := EpicProgress(b, nil, "rl4m9x")
	if done != 1 || total != 3 {
		t.Fatalf("progress = %d/%d, want 1/3", done, total)
	}
}

// A board that declares no terminal column truthfully reports zero
// progress; that is a reading of the configuration, not an error.
func TestEpicProgress_NoTerminalColumnYieldsZero(t *testing.T) {
	b := epicBoard(t, "")
	done, total := EpicProgress(b, nil, "rl4m9x")
	if done != 0 || total != 3 {
		t.Fatalf("progress = %d/%d, want 0/3", done, total)
	}
}

// archivedChild builds an ArchivedCard whose Epic and Column mirror the
// live cards epicBoard produces, so tests can drop children into the
// archive without diverging from the live fixture's shape.
func archivedChild(id, column, epic string) ArchivedCard {
	now := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	return ArchivedCard{
		Card: Card{
			ID: id, Title: "card " + id, Column: column, Tags: []string{},
			Epic: epic, CreatedAt: now, UpdatedAt: now,
		},
		ArchivedAt: now,
	}
}

// Archiving a done child must not change its epic's progress: the
// child moves from live to archived, but its column travels with it,
// so it still counts toward both done and total.
func TestEpicProgress_CountsArchivedChildren(t *testing.T) {
	b := epicBoard(t, "done")
	// aaaaa1 and bbbbb1 (both "todo") are archived from "done" —
	// simulating that they were moved to "done" and archived from
	// there, leaving only ccccc1 live in "done".
	b.Cards = []Card{
		b.Cards[0], // rl4m9x, the parent
		b.Cards[1], // ccccc1, live, done
		b.Cards[4], // loose1
	}
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{
			archivedChild("aaaaa1", "done", "rl4m9x"),
			archivedChild("bbbbb1", "done", "rl4m9x"),
		},
	}
	done, total := EpicProgress(b, a, "rl4m9x")
	if done != 3 || total != 3 {
		t.Fatalf("progress = %d/%d, want 3/3 — archiving must not change it", done, total)
	}
}

// An archived child from a non-terminal column counts toward total but
// never inflates done — the mirror of a live card in that column.
func TestEpicProgress_ArchivedFromNonTerminalColumnCountsOnlyTotal(t *testing.T) {
	b := epicBoard(t, "done")
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{
			archivedChild("zzzzz1", "todo", "rl4m9x"),
		},
	}
	done, total := EpicProgress(b, a, "rl4m9x")
	if done != 1 || total != 4 {
		t.Fatalf("progress = %d/%d, want 1/4", done, total)
	}
}

// A nil archive must reproduce EpicProgress's pre-archiving behaviour
// exactly — the guarantee every archive-free board relies on.
func TestEpicProgress_NilArchiveMatchesPreviousBehaviour(t *testing.T) {
	b := epicBoard(t, "done")
	done, total := EpicProgress(b, nil, "rl4m9x")
	if done != 1 || total != 3 {
		t.Fatalf("progress = %d/%d, want 1/3", done, total)
	}
}

// Done-ness is resolved at read time against the board's current
// terminal columns. If the column an archived child recorded is later
// un-marked (or removed), that child silently stops counting toward
// done while it keeps counting toward total — the documented caveat.
func TestEpicProgress_ArchivedChildStopsCountingWhenColumnDeleted(t *testing.T) {
	b := epicBoard(t, "done")
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{
			archivedChild("aaaaa1", "done", "rl4m9x"),
		},
	}
	b.Cards = []Card{b.Cards[0]} // only the parent remains live

	done, total := EpicProgress(b, a, "rl4m9x")
	if done != 1 || total != 1 {
		t.Fatalf("progress before column removal = %d/%d, want 1/1", done, total)
	}

	// Remove "done" the way `ezida columns rm` does: clear the
	// terminal flag and drop it from the declared columns.
	b.Board.SetDoneColumn("done", false)
	b.Board.Columns = []string{"todo"}

	done, total = EpicProgress(b, a, "rl4m9x")
	if done != 0 || total != 1 {
		t.Fatalf("progress after column removal = %d/%d, want 0/1", done, total)
	}
}

// ArchivedChildrenOf must return children in archive file order, the
// same convention ChildrenOf holds for the live board.
func TestArchivedChildrenOf_ArchiveFileOrder(t *testing.T) {
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards: []ArchivedCard{
			archivedChild("ccccc1", "done", "rl4m9x"),
			archivedChild("aaaaa1", "todo", "rl4m9x"),
			archivedChild("other1", "todo", "elsewhere"),
			archivedChild("bbbbb1", "todo", "rl4m9x"),
		},
	}
	children := ArchivedChildrenOf(a, "rl4m9x")
	want := []string{"ccccc1", "aaaaa1", "bbbbb1"}
	if len(children) != len(want) {
		t.Fatalf("got %d children, want %d", len(children), len(want))
	}
	for i, c := range children {
		if c.ID != want[i] {
			t.Fatalf("children = %v, want %v", children, want)
		}
	}
	if got := ArchivedChildrenOf(a, "loose1"); got != nil {
		t.Errorf("ArchivedChildrenOf(childless) = %v, want nil", got)
	}
	if got := ArchivedChildrenOf(a, ""); got != nil {
		t.Errorf("ArchivedChildrenOf(\"\") = %v, want nil", got)
	}
	if got := ArchivedChildrenOf(nil, "rl4m9x"); got != nil {
		t.Errorf("ArchivedChildrenOf(nil archive) = %v, want nil", got)
	}
}

func TestIsEpicAndParentOf(t *testing.T) {
	b := epicBoard(t, "done")
	if !IsEpic(b, nil, "rl4m9x") {
		t.Error("parent not reported as an epic")
	}
	if IsEpic(b, nil, "loose1") {
		t.Error("childless card reported as an epic")
	}
	if IsEpic(b, nil, "") {
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

// A card with no live children still counts as an epic when its only
// children are archived: they are filed away, not gone.
func TestIsEpic_TrueWithOnlyArchivedChildren(t *testing.T) {
	b := epicBoard(t, "done")
	b.Cards = []Card{b.Cards[0], b.Cards[4]} // parent + loose1, no live children
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards:         []ArchivedCard{archivedChild("aaaaa1", "todo", "rl4m9x")},
	}
	if !IsEpic(b, a, "rl4m9x") {
		t.Error("parent with only archived children not reported as an epic")
	}
	if IsEpic(b, nil, "rl4m9x") {
		t.Error("nil archive should not see the archived-only child")
	}
}

func TestCheckEpicTarget(t *testing.T) {
	b := epicBoard(t, "done")
	if err := CheckEpicTarget(b, nil, "loose1", "rl4m9x"); err != nil {
		t.Fatalf("legal target rejected: %v", err)
	}
	if err := CheckEpicTarget(b, nil, "loose1", ""); err != nil {
		t.Fatalf("empty target rejected: %v", err)
	}
	if err := CheckEpicTarget(b, nil, "loose1", "loose1"); err == nil {
		t.Error("self-reference accepted")
	}
	if err := CheckEpicTarget(b, nil, "loose1", "zzzzzz"); err == nil {
		t.Error("unknown target accepted")
	}
	// aaaaa1 is itself a child, so it may not become a parent.
	err := CheckEpicTarget(b, nil, "loose1", "aaaaa1")
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

// The mirror of the nested-target rule: rl4m9x has children, so
// giving it a parent would push them to a second level. The refusal
// must come from CheckEpicTarget, before anything is written — not
// from the whole-board Validate afterwards.
func TestCheckEpicTarget_ChildWithChildrenRefused(t *testing.T) {
	b := epicBoard(t, "done")
	err := CheckEpicTarget(b, nil, "rl4m9x", "loose1")
	if err == nil {
		t.Fatal("a card with children was given an epic")
	}
	var ie *InvalidEpicError
	if !asInvalidEpic(err, &ie) {
		t.Fatalf("error = %T, want *InvalidEpicError", err)
	}
	if ie.ID != "loose1" {
		t.Errorf("error names %q, want the refused target loose1", ie.ID)
	}
	if ie.Reason == "" {
		t.Error("refusal carried no reason")
	}
	// The board is a pure read for this call.
	if b.Cards[0].Epic != "" {
		t.Errorf("board mutated: rl4m9x now carries epic %q", b.Cards[0].Epic)
	}
	// A childless card is still free to acquire one.
	if err := CheckEpicTarget(b, nil, "loose1", "rl4m9x"); err != nil {
		t.Fatalf("childless card refused: %v", err)
	}
}

// Closes the nesting trap the design doc describes: a card whose only
// child is archived is still an epic, so giving it a parent of its own
// must be refused just as it would be for a live child.
func TestCheckEpicTarget_RefusesParentForCardWithOnlyArchivedChildren(t *testing.T) {
	b := epicBoard(t, "done")
	b.Cards = []Card{b.Cards[0], b.Cards[4]} // rl4m9x + loose1, no live children
	a := &Archive{
		SchemaVersion: SupportedSchemaVersion,
		Cards:         []ArchivedCard{archivedChild("aaaaa1", "todo", "rl4m9x")},
	}
	err := CheckEpicTarget(b, a, "rl4m9x", "loose1")
	if err == nil {
		t.Fatal("a card with only archived children was given an epic")
	}
	var ie *InvalidEpicError
	if !asInvalidEpic(err, &ie) {
		t.Fatalf("error = %T, want *InvalidEpicError", err)
	}
}

// A nil archive must reduce CheckEpicTarget to its pre-archiving,
// live-only reading: a card whose only children are archived is
// invisible to it, so the nesting refusal above does not fire.
func TestCheckEpicTarget_NilArchiveMatchesPreviousBehaviour(t *testing.T) {
	b := epicBoard(t, "done")
	b.Cards = []Card{b.Cards[0], b.Cards[4]} // rl4m9x + loose1, no live children
	if err := CheckEpicTarget(b, nil, "rl4m9x", "loose1"); err != nil {
		t.Fatalf("nil archive should not see archived-only children: %v", err)
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
