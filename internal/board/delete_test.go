package board

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func deleteTestBoard() *Board {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	return &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    []string{"todo", "done"},
			Priorities: []string{"low", "high"},
		},
		Cards: []Card{
			{ID: "aaaaaa", Title: "A", Column: "todo", CreatedAt: now, UpdatedAt: now},
			{ID: "bbbbbb", Title: "B", Column: "todo", CreatedAt: now, UpdatedAt: now},
			{ID: "cccccc", Title: "C", Column: "done", CreatedAt: now, UpdatedAt: now},
		},
	}
}

func TestDeleteCard_Success(t *testing.T) {
	b := deleteTestBoard()
	if err := DeleteCard(b, "bbbbbb"); err != nil {
		t.Fatalf("DeleteCard returned err = %v, want nil", err)
	}
	if len(b.Cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(b.Cards))
	}
	want := []string{"aaaaaa", "cccccc"}
	if got := cardIDs(b.Cards); !reflectStringSliceEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDeleteCard_PreservesOrder(t *testing.T) {
	b := deleteTestBoard()
	// Snapshot the cards we expect to survive.
	survivorA := b.Cards[0]
	survivorC := b.Cards[2]

	if err := DeleteCard(b, "bbbbbb"); err != nil {
		t.Fatalf("DeleteCard returned err = %v", err)
	}
	if len(b.Cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(b.Cards))
	}
	if b.Cards[0].ID != survivorA.ID || b.Cards[0].Title != survivorA.Title ||
		b.Cards[0].Column != survivorA.Column ||
		!b.Cards[0].CreatedAt.Equal(survivorA.CreatedAt) ||
		!b.Cards[0].UpdatedAt.Equal(survivorA.UpdatedAt) {
		t.Fatalf("survivor A mutated: got %+v, want %+v", b.Cards[0], survivorA)
	}
	if b.Cards[1].ID != survivorC.ID || b.Cards[1].Title != survivorC.Title ||
		b.Cards[1].Column != survivorC.Column ||
		!b.Cards[1].CreatedAt.Equal(survivorC.CreatedAt) ||
		!b.Cards[1].UpdatedAt.Equal(survivorC.UpdatedAt) {
		t.Fatalf("survivor C mutated: got %+v, want %+v", b.Cards[1], survivorC)
	}
}

func TestDeleteCard_UnknownIDReturnsNotFound(t *testing.T) {
	b := deleteTestBoard()
	err := DeleteCard(b, "zzzzzz")
	if err == nil {
		t.Fatalf("DeleteCard(unknown) returned nil err, want *CardNotFoundError")
	}
	var cnf *CardNotFoundError
	if !errors.As(err, &cnf) {
		t.Fatalf("got %T, want *CardNotFoundError", err)
	}
	if cnf.ID != "zzzzzz" {
		t.Fatalf("CardNotFoundError.ID = %q, want %q", cnf.ID, "zzzzzz")
	}
}

func TestDeleteCard_DoesNotMutateOnMiss(t *testing.T) {
	b := deleteTestBoard()
	beforeIDs := cardIDs(b.Cards)
	beforeLen := len(b.Cards)

	if err := DeleteCard(b, "zzzzzz"); err == nil {
		t.Fatalf("expected error, got nil")
	}

	if len(b.Cards) != beforeLen {
		t.Fatalf("len(b.Cards) = %d, want %d", len(b.Cards), beforeLen)
	}
	if got := cardIDs(b.Cards); !reflectStringSliceEqual(got, beforeIDs) {
		t.Fatalf("cards mutated on miss: got %v, want %v", got, beforeIDs)
	}
}

func TestDeleteCard_SingleCardBoardEndsEmpty(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	b := &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    []string{"todo"},
			Priorities: []string{"low"},
		},
		Cards: []Card{
			{ID: "aaaaaa", Title: "Only", Column: "todo", CreatedAt: now, UpdatedAt: now},
		},
	}
	if err := DeleteCard(b, "aaaaaa"); err != nil {
		t.Fatalf("DeleteCard returned err = %v", err)
	}
	if len(b.Cards) != 0 {
		t.Fatalf("len(b.Cards) = %d, want 0", len(b.Cards))
	}
}

func TestDeleteCard_DoesNotTouchBoardConfig(t *testing.T) {
	b := deleteTestBoard()
	beforeSchema := b.SchemaVersion
	beforeCols := append([]string(nil), b.Board.Columns...)
	beforePris := append([]string(nil), b.Board.Priorities...)

	// success
	if err := DeleteCard(b, "aaaaaa"); err != nil {
		t.Fatalf("DeleteCard returned err = %v", err)
	}
	if b.SchemaVersion != beforeSchema {
		t.Fatalf("schema_version changed: got %d, want %d", b.SchemaVersion, beforeSchema)
	}
	if !reflectStringSliceEqual(b.Board.Columns, beforeCols) {
		t.Fatalf("columns mutated: got %v, want %v", b.Board.Columns, beforeCols)
	}
	if !reflectStringSliceEqual(b.Board.Priorities, beforePris) {
		t.Fatalf("priorities mutated: got %v, want %v", b.Board.Priorities, beforePris)
	}

	// failure
	if err := DeleteCard(b, "zzzzzz"); err == nil {
		t.Fatalf("expected error on miss")
	}
	if b.SchemaVersion != beforeSchema {
		t.Fatalf("schema_version changed on miss: got %d, want %d", b.SchemaVersion, beforeSchema)
	}
	if !reflectStringSliceEqual(b.Board.Columns, beforeCols) {
		t.Fatalf("columns mutated on miss: got %v, want %v", b.Board.Columns, beforeCols)
	}
	if !reflectStringSliceEqual(b.Board.Priorities, beforePris) {
		t.Fatalf("priorities mutated on miss: got %v, want %v", b.Board.Priorities, beforePris)
	}
}

func TestDeleteCard_IdempotentAtFileLayer(t *testing.T) {
	tmpDir := t.TempDir()
	src, err := Load(filepath.Join("testdata", "valid.toml"))
	if err != nil {
		t.Fatalf("Load(valid.toml): %v", err)
	}
	if len(src.Cards) == 0 {
		t.Fatalf("valid.toml has no cards to delete")
	}
	target := src.Cards[0].ID

	out := filepath.Join(tmpDir, "kanban.toml")
	// First delete + Save.
	if err := DeleteCard(src, target); err != nil {
		t.Fatalf("first DeleteCard: %v", err)
	}
	if err := Save(out, src); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	// Reload from disk and try a second delete of the same id.
	reloaded, err := Load(out)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if err := DeleteCard(reloaded, target); err == nil {
		t.Fatalf("second DeleteCard returned nil, want *CardNotFoundError")
	} else {
		var cnf *CardNotFoundError
		if !errors.As(err, &cnf) {
			t.Fatalf("got %T, want *CardNotFoundError", err)
		}
	}
}
