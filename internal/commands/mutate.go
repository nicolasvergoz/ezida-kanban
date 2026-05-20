package commands

import (
	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// mutateAndSave loads the board at path, applies the mutate closure to
// the in-memory board, then saves it atomically. The closure returns
// the affected card (echoed by JSON-mode mutating commands) and any
// validation/lookup error.
//
// If the closure returns an error, no write is attempted. board.Save
// already runs board.Validate internally (per the P1 contract) so the
// "re-validate before write" invariant is inherited.
func mutateAndSave(path string, mutate func(b *board.Board) (board.Card, error)) (board.Card, error) {
	b, err := board.Load(path)
	if err != nil {
		return board.Card{}, err
	}
	c, err := mutate(b)
	if err != nil {
		return board.Card{}, err
	}
	if err := board.Save(path, b); err != nil {
		return board.Card{}, err
	}
	return c, nil
}

// indexCardByID returns the index of the card with the given ID in
// cards, or -1 if no card matches. Used by `move` and `rm`.
func indexCardByID(cards []board.Card, id string) int {
	for i := range cards {
		if cards[i].ID == id {
			return i
		}
	}
	return -1
}
