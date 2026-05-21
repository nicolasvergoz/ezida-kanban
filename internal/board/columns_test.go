package board

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// --- helpers ---------------------------------------------------------------

func newColumnsFixture(cols []string) *Board {
	b := &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    append([]string(nil), cols...),
			Priorities: []string{"low", "medium", "high"},
		},
	}
	return b
}

func newColumnsFixtureWithCards(cols []string, cards []Card) *Board {
	b := newColumnsFixture(cols)
	b.Cards = append([]Card(nil), cards...)
	return b
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- AddColumn -------------------------------------------------------------

func TestAddColumn_Success(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	if err := AddColumn(b, "review"); err != nil {
		t.Fatalf("AddColumn returned %v, want nil", err)
	}
	want := []string{"todo", "done", "review"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestAddColumn_TrimsWhitespace(t *testing.T) {
	b := newColumnsFixture([]string{"todo"})
	if err := AddColumn(b, "  review  "); err != nil {
		t.Fatalf("AddColumn returned %v, want nil", err)
	}
	want := []string{"todo", "review"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestAddColumn_EmptyRejected(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	before := append([]string(nil), b.Board.Columns...)
	err := AddColumn(b, "")
	var ene *EmptyColumnNameError
	if !errors.As(err, &ene) {
		t.Fatalf("err = %v, want *EmptyColumnNameError", err)
	}
	if !sameStringSlice(b.Board.Columns, before) {
		t.Fatalf("columns mutated: %v, want %v", b.Board.Columns, before)
	}
}

func TestAddColumn_WhitespaceOnlyRejected(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	before := append([]string(nil), b.Board.Columns...)
	err := AddColumn(b, "   ")
	var ene *EmptyColumnNameError
	if !errors.As(err, &ene) {
		t.Fatalf("err = %v, want *EmptyColumnNameError", err)
	}
	if !sameStringSlice(b.Board.Columns, before) {
		t.Fatalf("columns mutated: %v, want %v", b.Board.Columns, before)
	}
}

func TestAddColumn_DuplicateRejected(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	before := append([]string(nil), b.Board.Columns...)
	err := AddColumn(b, "todo")
	var caee *ColumnAlreadyExistsError
	if !errors.As(err, &caee) {
		t.Fatalf("err = %v, want *ColumnAlreadyExistsError", err)
	}
	if caee.Name != "todo" {
		t.Fatalf("error.Name = %q, want %q", caee.Name, "todo")
	}
	if !sameStringSlice(b.Board.Columns, before) {
		t.Fatalf("columns mutated: %v, want %v", b.Board.Columns, before)
	}
}

// --- RenameColumn ----------------------------------------------------------

func TestRenameColumn_Success_PropagatesToCards(t *testing.T) {
	ts := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	b := newColumnsFixtureWithCards(
		[]string{"todo", "done"},
		[]Card{
			{ID: "a", Title: "A", Column: "todo", CreatedAt: ts, UpdatedAt: ts},
			{ID: "b", Title: "B", Column: "todo", CreatedAt: ts, UpdatedAt: ts},
			{ID: "c", Title: "C", Column: "todo", CreatedAt: ts, UpdatedAt: ts},
		},
	)
	if err := RenameColumn(b, "todo", "backlog"); err != nil {
		t.Fatalf("RenameColumn returned %v, want nil", err)
	}
	want := []string{"backlog", "done"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
	for _, c := range b.Cards {
		if c.Column != "backlog" {
			t.Fatalf("card %q has column %q, want %q", c.ID, c.Column, "backlog")
		}
	}
}

func TestRenameColumn_SameNameIsNoop(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	beforeCols := append([]string(nil), b.Board.Columns...)
	beforeCards := append([]Card(nil), b.Cards...)
	if err := RenameColumn(b, "todo", "todo"); err != nil {
		t.Fatalf("RenameColumn returned %v, want nil", err)
	}
	if !sameStringSlice(b.Board.Columns, beforeCols) {
		t.Fatalf("columns mutated: %v, want %v", b.Board.Columns, beforeCols)
	}
	if !reflect.DeepEqual(b.Cards, beforeCards) {
		t.Fatalf("cards mutated")
	}
}

func TestRenameColumn_UnknownFromReturnsNotFound(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	before := append([]string(nil), b.Board.Columns...)
	err := RenameColumn(b, "ghost", "backlog")
	var cnfe *ColumnNotFoundError
	if !errors.As(err, &cnfe) {
		t.Fatalf("err = %v, want *ColumnNotFoundError", err)
	}
	if !sameStringSlice(b.Board.Columns, before) {
		t.Fatalf("columns mutated: %v", b.Board.Columns)
	}
}

func TestRenameColumn_DuplicateToReturnsExists(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	before := append([]string(nil), b.Board.Columns...)
	err := RenameColumn(b, "todo", "done")
	var caee *ColumnAlreadyExistsError
	if !errors.As(err, &caee) {
		t.Fatalf("err = %v, want *ColumnAlreadyExistsError", err)
	}
	if caee.Name != "done" {
		t.Fatalf("error.Name = %q, want %q", caee.Name, "done")
	}
	if !sameStringSlice(b.Board.Columns, before) {
		t.Fatalf("columns mutated: %v", b.Board.Columns)
	}
}

func TestRenameColumn_EmptyToRejected(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	before := append([]string(nil), b.Board.Columns...)
	for _, name := range []string{"", "   "} {
		err := RenameColumn(b, "todo", name)
		var ene *EmptyColumnNameError
		if !errors.As(err, &ene) {
			t.Fatalf("err(%q) = %v, want *EmptyColumnNameError", name, err)
		}
	}
	if !sameStringSlice(b.Board.Columns, before) {
		t.Fatalf("columns mutated: %v", b.Board.Columns)
	}
}

func TestRenameColumn_DoesNotRefreshUpdatedAt(t *testing.T) {
	original := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	b := newColumnsFixtureWithCards(
		[]string{"todo", "done"},
		[]Card{
			{ID: "a", Title: "A", Column: "todo", CreatedAt: original, UpdatedAt: original},
		},
	)
	if err := RenameColumn(b, "todo", "backlog"); err != nil {
		t.Fatalf("RenameColumn returned %v", err)
	}
	if !b.Cards[0].UpdatedAt.Equal(original) {
		t.Fatalf("UpdatedAt = %s, want %s (unchanged)", b.Cards[0].UpdatedAt, original)
	}
}

// --- DeleteColumn ----------------------------------------------------------

func TestDeleteColumn_Success(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done", "review"})
	if err := DeleteColumn(b, "review"); err != nil {
		t.Fatalf("DeleteColumn returned %v, want nil", err)
	}
	want := []string{"todo", "done"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestDeleteColumn_UnknownReturnsNotFound(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "done"})
	before := append([]string(nil), b.Board.Columns...)
	err := DeleteColumn(b, "ghost")
	var cnfe *ColumnNotFoundError
	if !errors.As(err, &cnfe) {
		t.Fatalf("err = %v, want *ColumnNotFoundError", err)
	}
	if cnfe.Column != "ghost" {
		t.Fatalf("error.Column = %q, want %q", cnfe.Column, "ghost")
	}
	if !sameStringSlice(b.Board.Columns, before) {
		t.Fatalf("columns mutated: %v", b.Board.Columns)
	}
}

func TestDeleteColumn_LastColumnRefused(t *testing.T) {
	b := newColumnsFixture([]string{"todo"})
	err := DeleteColumn(b, "todo")
	var cdle *CannotDeleteLastColumnError
	if !errors.As(err, &cdle) {
		t.Fatalf("err = %v, want *CannotDeleteLastColumnError", err)
	}
	if cdle.Name != "todo" {
		t.Fatalf("error.Name = %q, want %q", cdle.Name, "todo")
	}
	if !sameStringSlice(b.Board.Columns, []string{"todo"}) {
		t.Fatalf("columns mutated: %v", b.Board.Columns)
	}
}

func TestDeleteColumn_CardsBlockDelete(t *testing.T) {
	ts := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	b := newColumnsFixtureWithCards(
		[]string{"todo", "done"},
		[]Card{
			{ID: "aaaaaa", Title: "First", Column: "todo", CreatedAt: ts, UpdatedAt: ts},
			{ID: "bbbbbb", Title: "Second", Column: "todo", CreatedAt: ts, UpdatedAt: ts},
		},
	)
	beforeCols := append([]string(nil), b.Board.Columns...)
	beforeCards := append([]Card(nil), b.Cards...)
	err := DeleteColumn(b, "todo")
	var che *ColumnHasCardsError
	if !errors.As(err, &che) {
		t.Fatalf("err = %v, want *ColumnHasCardsError", err)
	}
	if che.Name != "todo" {
		t.Fatalf("error.Name = %q, want %q", che.Name, "todo")
	}
	if len(che.Cards) != 2 {
		t.Fatalf("len(Cards) = %d, want 2", len(che.Cards))
	}
	if che.Cards[0].ID != "aaaaaa" || che.Cards[0].Title != "First" {
		t.Fatalf("Cards[0] = %+v", che.Cards[0])
	}
	if che.Cards[1].ID != "bbbbbb" || che.Cards[1].Title != "Second" {
		t.Fatalf("Cards[1] = %+v", che.Cards[1])
	}
	if !sameStringSlice(b.Board.Columns, beforeCols) {
		t.Fatalf("columns mutated")
	}
	if !reflect.DeepEqual(b.Cards, beforeCards) {
		t.Fatalf("cards mutated")
	}
}

func TestDeleteColumn_PreservesColumnOrder(t *testing.T) {
	b := newColumnsFixture([]string{"a", "b", "c"})
	if err := DeleteColumn(b, "b"); err != nil {
		t.Fatalf("DeleteColumn returned %v", err)
	}
	want := []string{"a", "c"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

// --- MoveColumn ------------------------------------------------------------

func TestMoveColumn_ToFirst(t *testing.T) {
	b := newColumnsFixture([]string{"todo", "ongoing", "done"})
	if err := MoveColumn(b, "done", 0); err != nil {
		t.Fatalf("MoveColumn returned %v", err)
	}
	want := []string{"done", "todo", "ongoing"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestMoveColumn_ToMiddle(t *testing.T) {
	b := newColumnsFixture([]string{"a", "b", "c", "d"})
	if err := MoveColumn(b, "d", 1); err != nil {
		t.Fatalf("MoveColumn returned %v", err)
	}
	want := []string{"a", "d", "b", "c"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestMoveColumn_ToLast(t *testing.T) {
	b := newColumnsFixture([]string{"a", "b", "c"})
	if err := MoveColumn(b, "a", 2); err != nil {
		t.Fatalf("MoveColumn returned %v", err)
	}
	want := []string{"b", "c", "a"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestMoveColumn_PositionPastEndClamps(t *testing.T) {
	b := newColumnsFixture([]string{"a", "b", "c"})
	if err := MoveColumn(b, "a", 999); err != nil {
		t.Fatalf("MoveColumn returned %v", err)
	}
	want := []string{"b", "c", "a"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestMoveColumn_NegativePositionClamps(t *testing.T) {
	b := newColumnsFixture([]string{"a", "b", "c"})
	if err := MoveColumn(b, "c", -5); err != nil {
		t.Fatalf("MoveColumn returned %v", err)
	}
	want := []string{"c", "a", "b"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestMoveColumn_NoOpWhenAlreadyAtTarget(t *testing.T) {
	b := newColumnsFixture([]string{"a", "b", "c"})
	if err := MoveColumn(b, "b", 1); err != nil {
		t.Fatalf("MoveColumn returned %v", err)
	}
	want := []string{"a", "b", "c"}
	if !sameStringSlice(b.Board.Columns, want) {
		t.Fatalf("columns = %v, want %v", b.Board.Columns, want)
	}
}

func TestMoveColumn_UnknownReturnsNotFound(t *testing.T) {
	b := newColumnsFixture([]string{"a", "b", "c"})
	before := append([]string(nil), b.Board.Columns...)
	err := MoveColumn(b, "ghost", 0)
	var cnfe *ColumnNotFoundError
	if !errors.As(err, &cnfe) {
		t.Fatalf("err = %v, want *ColumnNotFoundError", err)
	}
	if !sameStringSlice(b.Board.Columns, before) {
		t.Fatalf("columns mutated: %v", b.Board.Columns)
	}
}

func TestMoveColumn_CardsUntouched(t *testing.T) {
	ts := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	b := newColumnsFixtureWithCards(
		[]string{"a", "b", "c"},
		[]Card{
			{ID: "aaaaaa", Title: "A1", Column: "a", CreatedAt: ts, UpdatedAt: ts},
			{ID: "bbbbbb", Title: "B1", Column: "b", CreatedAt: ts, UpdatedAt: ts},
			{ID: "cccccc", Title: "C1", Column: "c", CreatedAt: ts, UpdatedAt: ts},
		},
	)
	beforeCards := append([]Card(nil), b.Cards...)
	if err := MoveColumn(b, "c", 0); err != nil {
		t.Fatalf("MoveColumn returned %v", err)
	}
	if !reflect.DeepEqual(b.Cards, beforeCards) {
		t.Fatalf("cards mutated by MoveColumn")
	}
}
