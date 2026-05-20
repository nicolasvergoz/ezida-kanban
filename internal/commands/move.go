package commands

import (
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// NewMoveCmd builds the `ezida move <id> <column>` command.
func NewMoveCmd(jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <id> <column>",
		Short: "Move a card to a different column",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMove(cmd, BoardPath, args[0], args[1], *jsonOut)
		},
	}
	return cmd
}

// runMove is the testable run body for `ezida move`.
func runMove(cmd *cobra.Command, path, id, column string, asJSON bool) error {
	card, err := mutateAndSave(path, func(b *board.Board) (board.Card, error) {
		if !slices.Contains(b.Board.Columns, column) {
			return board.Card{}, &ColumnNotFoundError{Name: column}
		}
		idx := indexCardByID(b.Cards, id)
		if idx < 0 {
			return board.Card{}, &CardNotFoundError{ID: id}
		}
		c := b.Cards[idx]
		b.Cards = slices.Delete(b.Cards, idx, idx+1)
		c.Column = column
		c.UpdatedAt = nowFunc().UTC().Truncate(time.Second)
		board.AppendCardToColumn(b, c)
		return c, nil
	})
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if asJSON {
		_, err = out.Write(output.JSONCard(card))
		return err
	}
	_, err = fmt.Fprintln(out, card.ID)
	return err
}
