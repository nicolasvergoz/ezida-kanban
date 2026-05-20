package commands

import (
	"errors"
	"os"
	"testing"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

func TestMutateAndSave_HappyPath(t *testing.T) {
	path := copyFixture(t)
	pre, _ := os.ReadFile(path)
	card, err := mutateAndSave(path, func(b *board.Board) (board.Card, error) {
		// Trivial mutation: bump no-op (still must pass validate).
		return b.Cards[0], nil
	})
	if err != nil {
		t.Fatalf("mutateAndSave: %v", err)
	}
	if card.ID == "" {
		t.Errorf("empty card returned")
	}
	// File should still be loadable.
	if _, err := board.Load(path); err != nil {
		t.Errorf("post-save load: %v", err)
	}
	post, _ := os.ReadFile(path)
	// We do not require byte equality (re-marshal may reorder), only that
	// the board still loads and validates.
	_ = pre
	_ = post
}

func TestMutateAndSave_ClosureError_NoWrite(t *testing.T) {
	path := copyFixture(t)
	pre, _ := os.ReadFile(path)
	sentinel := errors.New("closure failure")
	_, err := mutateAndSave(path, func(b *board.Board) (board.Card, error) {
		return board.Card{}, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v, want %v", err, sentinel)
	}
	post, _ := os.ReadFile(path)
	if string(pre) != string(post) {
		t.Errorf("file changed despite closure error")
	}
}

func TestIndexCardByID(t *testing.T) {
	cards := []board.Card{
		{ID: "a3f2k9"},
		{ID: "b7m1p4"},
	}
	if idx := indexCardByID(cards, "b7m1p4"); idx != 1 {
		t.Errorf("got %d, want 1", idx)
	}
	if idx := indexCardByID(cards, "zzzzzz"); idx != -1 {
		t.Errorf("got %d, want -1", idx)
	}
}
