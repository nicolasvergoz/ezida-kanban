package board

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func newTestBoard() *Board {
	created := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	return &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    []string{"backlog", "todo", "done"},
			Priorities: []string{"low", "medium", "high"},
		},
		Cards: []Card{
			{ID: "aaaaaa", Title: "Standalone", Column: "todo", CreatedAt: created, UpdatedAt: created},
			{ID: "rl4m9x", Title: "Epic parent", Column: "todo", CreatedAt: created, UpdatedAt: created, Color: "#22c55e"},
			{ID: "f20wbo", Title: "Child 1", Column: "todo", CreatedAt: created, UpdatedAt: created, Epic: "rl4m9x"},
			{ID: "wrshlo", Title: "Child 2", Column: "done", CreatedAt: created, UpdatedAt: created, Epic: "rl4m9x"},
			{ID: "bbbbbb", Title: "Other", Column: "backlog", CreatedAt: created, UpdatedAt: created},
		},
	}
}

func emptyArchive() *Archive {
	return &Archive{SchemaVersion: SupportedSchemaVersion, Cards: []ArchivedCard{}}
}

// --- ArchiveCard ---------------------------------------------------------

func TestArchiveCard_CascadesChildren(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	at := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)

	archived, err := ArchiveCard(b, a, "rl4m9x", at)
	if err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}
	if len(archived) != 3 {
		t.Fatalf("archived %d cards, want 3 (parent + 2 children)", len(archived))
	}
	if archived[0].ID != "rl4m9x" {
		t.Fatalf("archived[0].ID = %q, want rl4m9x (parent first)", archived[0].ID)
	}
	gotIDs := map[string]bool{}
	for _, c := range archived {
		gotIDs[c.ID] = true
	}
	for _, want := range []string{"rl4m9x", "f20wbo", "wrshlo"} {
		if !gotIDs[want] {
			t.Fatalf("archived set missing %q", want)
		}
	}

	for _, c := range b.Cards {
		if c.ID == "rl4m9x" || c.ID == "f20wbo" || c.ID == "wrshlo" {
			t.Fatalf("card %q still on the board after cascade archive", c.ID)
		}
	}
	if len(b.Cards) != 2 {
		t.Fatalf("board has %d cards left, want 2 (aaaaaa, bbbbbb)", len(b.Cards))
	}

	if len(a.Cards) != 3 {
		t.Fatalf("archive has %d cards, want 3", len(a.Cards))
	}
	if a.Cards[0].ID != "rl4m9x" {
		t.Fatalf("archive head = %q, want rl4m9x (parent first)", a.Cards[0].ID)
	}
}

func TestArchiveCard_SharesOneTimestamp(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	at := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)

	archived, err := ArchiveCard(b, a, "rl4m9x", at)
	if err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}
	for _, c := range archived {
		if !c.ArchivedAt.Equal(at) {
			t.Fatalf("card %q ArchivedAt = %v, want %v", c.ID, c.ArchivedAt, at)
		}
	}
}

func TestArchiveCard_DoesNotTouchUpdatedAt(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	var before time.Time
	for _, c := range b.Cards {
		if c.ID == "aaaaaa" {
			before = c.UpdatedAt
		}
	}
	if _, err := ArchiveCard(b, a, "aaaaaa", time.Now().UTC()); err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}
	if !a.Cards[0].UpdatedAt.Equal(before) {
		t.Fatalf("UpdatedAt changed: got %v, want %v", a.Cards[0].UpdatedAt, before)
	}
}

func TestArchiveCard_LoneChildKeepsEpicString(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	if _, err := ArchiveCard(b, a, "f20wbo", time.Now().UTC()); err != nil {
		t.Fatalf("ArchiveCard: %v", err)
	}
	if a.Cards[0].Epic != "rl4m9x" {
		t.Fatalf("archived lone child Epic = %q, want rl4m9x", a.Cards[0].Epic)
	}
	found := false
	for _, c := range b.Cards {
		if c.ID == "rl4m9x" {
			found = true
		}
	}
	if !found {
		t.Fatal("parent rl4m9x must remain on the board")
	}
}

func TestArchiveCard_UnknownID(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	_, err := ArchiveCard(b, a, "zzzzzz", time.Now().UTC())
	var cnf *CardNotFoundError
	if !errors.As(err, &cnf) {
		t.Fatalf("ArchiveCard(unknown) err = %v, want *CardNotFoundError", err)
	}
}

// --- ArchiveColumn ---------------------------------------------------------

func TestArchiveColumn_CascadeOutsideColumnIsReportedSeparately(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	at := time.Now().UTC()

	direct, cascaded, err := ArchiveColumn(b, a, "todo", at)
	if err != nil {
		t.Fatalf("ArchiveColumn: %v", err)
	}
	// todo holds: aaaaaa, rl4m9x, f20wbo
	if len(direct) != 3 {
		t.Fatalf("direct = %d cards, want 3", len(direct))
	}
	// wrshlo is rl4m9x's child but lives in done — cascaded.
	if len(cascaded) != 1 || cascaded[0].ID != "wrshlo" {
		t.Fatalf("cascaded = %v, want [wrshlo]", cascaded)
	}
	for _, c := range b.Cards {
		if c.ID == "aaaaaa" || c.ID == "rl4m9x" || c.ID == "f20wbo" || c.ID == "wrshlo" {
			t.Fatalf("card %q still on board after ArchiveColumn", c.ID)
		}
	}
	if len(b.Cards) != 1 || b.Cards[0].ID != "bbbbbb" {
		t.Fatalf("board = %v, want only bbbbbb left", b.Cards)
	}
}

func TestArchiveColumn_LeavesColumnInPlace(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	if _, _, err := ArchiveColumn(b, a, "todo", time.Now().UTC()); err != nil {
		t.Fatalf("ArchiveColumn: %v", err)
	}
	found := false
	for _, col := range b.Board.Columns {
		if col == "todo" {
			found = true
		}
	}
	if !found {
		t.Fatal("column 'todo' must remain in [board].columns")
	}
}

func TestArchiveColumn_EmptyColumnIsNoOp(t *testing.T) {
	b := newTestBoard()
	b.Board.Columns = append(b.Board.Columns, "review")
	a := emptyArchive()
	before := append([]Card{}, b.Cards...)

	direct, cascaded, err := ArchiveColumn(b, a, "review", time.Now().UTC())
	if err != nil {
		t.Fatalf("ArchiveColumn(empty column): %v", err)
	}
	if len(direct) != 0 || len(cascaded) != 0 {
		t.Fatalf("expected no cards archived, got direct=%v cascaded=%v", direct, cascaded)
	}
	if !reflect.DeepEqual(before, b.Cards) {
		t.Fatal("board cards mutated by an empty-column archive")
	}
	if len(a.Cards) != 0 {
		t.Fatal("archive mutated by an empty-column archive")
	}
}

func TestArchiveColumn_UnknownColumn(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	_, _, err := ArchiveColumn(b, a, "ghost", time.Now().UTC())
	var cnf *ColumnNotFoundError
	if !errors.As(err, &cnf) {
		t.Fatalf("ArchiveColumn(unknown) err = %v, want *ColumnNotFoundError", err)
	}
}

// --- UnarchiveCard ---------------------------------------------------------

func archivedBoardAndArchive(t *testing.T) (*Board, *Archive) {
	t.Helper()
	b := newTestBoard()
	a := emptyArchive()
	if _, err := ArchiveCard(b, a, "rl4m9x", time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("setup ArchiveCard: %v", err)
	}
	return b, a
}

func TestUnarchiveCard_RestoresCascadeInBoardOrder(t *testing.T) {
	b, a := archivedBoardAndArchive(t)

	restored, orphaned, relocated, err := UnarchiveCard(b, a, "rl4m9x", "")
	if err != nil {
		t.Fatalf("UnarchiveCard: %v", err)
	}
	if len(restored) != 3 {
		t.Fatalf("restored %d cards, want 3", len(restored))
	}
	if len(orphaned) != 0 {
		t.Fatalf("orphaned = %v, want none", orphaned)
	}
	if relocated {
		t.Fatal("relocated = true, want false (todo still exists)")
	}
	if len(a.Cards) != 0 {
		t.Fatalf("archive still has %d cards after full restore", len(a.Cards))
	}

	// The parent must sit above its children within "todo" (parent
	// archived-first => restored last => ends up on top).
	var todoOrder []string
	for _, c := range b.Cards {
		if c.Column == "todo" {
			todoOrder = append(todoOrder, c.ID)
		}
	}
	parentPos, child1Pos := -1, -1
	for i, id := range todoOrder {
		if id == "rl4m9x" {
			parentPos = i
		}
		if id == "f20wbo" {
			child1Pos = i
		}
	}
	if parentPos < 0 || child1Pos < 0 || parentPos > child1Pos {
		t.Fatalf("todo order = %v, want parent before its todo-column child", todoOrder)
	}
}

func TestUnarchiveCard_ClearsEpicWhenParentGone(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	// Archive the lone child; its parent rl4m9x stays live, then also
	// archive the parent separately (children field on the archived
	// child still points at rl4m9x). To exercise "parent truly gone",
	// remove rl4m9x from the live board's ids entirely by archiving it
	// too under a different operation, then unarchive only the child.
	if _, err := ArchiveCard(b, a, "f20wbo", time.Now().UTC()); err != nil {
		t.Fatalf("archive child: %v", err)
	}
	if _, err := ArchiveCard(b, a, "rl4m9x", time.Now().UTC()); err != nil {
		t.Fatalf("archive parent: %v", err)
	}
	// Now restore only the child (wrshlo is rl4m9x's other child and
	// will cascade with it, but f20wbo is independent since it was
	// archived in its own operation and is not tied to whichever
	// unarchive we call). Restore f20wbo alone.
	restored, orphaned, _, err := UnarchiveCard(b, a, "f20wbo", "")
	if err != nil {
		t.Fatalf("UnarchiveCard(f20wbo): %v", err)
	}
	if len(restored) != 1 {
		t.Fatalf("restored = %v, want exactly 1 card", restored)
	}
	if restored[0].Epic != "" {
		t.Fatalf("restored card Epic = %q, want cleared (parent not restored)", restored[0].Epic)
	}
	if len(orphaned) != 1 || orphaned[0] != "f20wbo" {
		t.Fatalf("orphaned = %v, want [f20wbo]", orphaned)
	}
}

func TestUnarchiveCard_FallsBackToFirstColumn(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	if _, err := ArchiveCard(b, a, "aaaaaa", time.Now().UTC()); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// Remove the card's stored column ("todo") from the board.
	b.Board.Columns = []string{"backlog", "done"}

	restored, _, relocated, err := UnarchiveCard(b, a, "aaaaaa", "")
	if err != nil {
		t.Fatalf("UnarchiveCard: %v", err)
	}
	if !relocated {
		t.Fatal("relocated = false, want true (stored column gone)")
	}
	if restored[0].Column != "backlog" {
		t.Fatalf("restored column = %q, want backlog (board's first column)", restored[0].Column)
	}
}

func TestUnarchiveCard_ColumnOverride(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	if _, err := ArchiveCard(b, a, "aaaaaa", time.Now().UTC()); err != nil {
		t.Fatalf("archive: %v", err)
	}
	restored, _, relocated, err := UnarchiveCard(b, a, "aaaaaa", "done")
	if err != nil {
		t.Fatalf("UnarchiveCard: %v", err)
	}
	if restored[0].Column != "done" {
		t.Fatalf("restored column = %q, want done", restored[0].Column)
	}
	if relocated {
		t.Fatal("relocated should not be set when an explicit column is given")
	}
}

func TestUnarchiveCard_UnknownExplicitColumn(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	if _, err := ArchiveCard(b, a, "aaaaaa", time.Now().UTC()); err != nil {
		t.Fatalf("archive: %v", err)
	}
	_, _, _, err := UnarchiveCard(b, a, "aaaaaa", "ghost")
	var cnf *ColumnNotFoundError
	if !errors.As(err, &cnf) {
		t.Fatalf("err = %v, want *ColumnNotFoundError", err)
	}
	if len(a.Cards) != 1 {
		t.Fatal("archive mutated despite unknown column error")
	}
}

func TestUnarchiveCard_RefusesIDCollision(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	if _, err := ArchiveCard(b, a, "aaaaaa", time.Now().UTC()); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// Re-add a live card with the same id.
	b.Cards = append(b.Cards, Card{ID: "aaaaaa", Title: "New", Column: "todo", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})

	before := append([]ArchivedCard{}, a.Cards...)
	_, _, _, err := UnarchiveCard(b, a, "aaaaaa", "")
	var idc *IDCollisionError
	if !errors.As(err, &idc) {
		t.Fatalf("err = %v, want *IDCollisionError", err)
	}
	if !reflect.DeepEqual(before, a.Cards) {
		t.Fatal("archive mutated despite ID_COLLISION")
	}
}

func TestUnarchiveCard_UnknownID(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	_, _, _, err := UnarchiveCard(b, a, "zzzzzz", "")
	var cna *CardNotArchivedError
	if !errors.As(err, &cna) {
		t.Fatalf("err = %v, want *CardNotArchivedError", err)
	}
}

// --- Round trip -------------------------------------------------------------

func TestArchiveUnarchive_RoundTripIsIdentity(t *testing.T) {
	b := newTestBoard()
	a := emptyArchive()
	original := append([]Card{}, b.Cards...)

	if _, err := ArchiveCard(b, a, "aaaaaa", time.Now().UTC()); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, _, _, err := UnarchiveCard(b, a, "aaaaaa", ""); err != nil {
		t.Fatalf("unarchive: %v", err)
	}

	if len(b.Cards) != len(original) {
		t.Fatalf("board has %d cards after round trip, want %d", len(b.Cards), len(original))
	}
	byID := make(map[string]Card, len(b.Cards))
	for _, c := range b.Cards {
		byID[c.ID] = c
	}
	for _, want := range original {
		got, ok := byID[want.ID]
		if !ok {
			t.Fatalf("card %q missing after round trip", want.ID)
		}
		if got.Title != want.Title || got.Column != want.Column || got.Description != want.Description ||
			!got.CreatedAt.Equal(want.CreatedAt) || !got.UpdatedAt.Equal(want.UpdatedAt) ||
			got.Priority != want.Priority || got.Epic != want.Epic || got.Color != want.Color {
			t.Fatalf("card %q round trip mismatch: got %+v, want %+v", want.ID, got, want)
		}
	}
	if len(a.Cards) != 0 {
		t.Fatalf("archive has %d cards after full round trip, want 0", len(a.Cards))
	}
}
