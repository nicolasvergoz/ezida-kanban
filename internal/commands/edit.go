package commands

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// editFlags is the pointer-shaped collection of edit flags. A nil field
// means "flag was not passed"; a non-nil pointer to "" means the user
// explicitly passed an empty value (legal for description and priority;
// rejected for title).
type editFlags struct {
	title       *string
	description *string
	priority    *string
	tags        *string
	column      *string
}

// any reports whether at least one field was passed.
func (f editFlags) any() bool {
	return f.title != nil || f.description != nil || f.priority != nil ||
		f.tags != nil || f.column != nil
}

// editFlagState holds the raw string targets used by cobra. The
// converter buildEditFlags consumes this struct + cmd to produce an
// editFlags whose pointers reflect Flags().Changed.
type editFlagState struct {
	title       string
	description string
	priority    string
	tags        string
	column      string
}

// buildEditFlags walks cobra's "was flag passed?" map and produces an
// editFlags pointer struct accordingly. Unset flags resolve to nil;
// set flags resolve to a pointer to the (possibly empty) string value.
func buildEditFlags(cmd *cobra.Command, raw editFlagState) editFlags {
	var f editFlags
	if cmd.Flags().Changed("title") {
		v := raw.title
		f.title = &v
	}
	if cmd.Flags().Changed("description") {
		v := raw.description
		f.description = &v
	}
	if cmd.Flags().Changed("priority") {
		v := raw.priority
		f.priority = &v
	}
	if cmd.Flags().Changed("tags") {
		v := raw.tags
		f.tags = &v
	}
	if cmd.Flags().Changed("column") {
		v := raw.column
		f.column = &v
	}
	return f
}

// NewEditCmd builds the `ezida edit <id>` command.
func NewEditCmd(jsonOut *bool) *cobra.Command {
	state := editFlagState{}
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Update one or more fields of an existing card",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := buildEditFlags(cmd, state)
			return runEdit(cmd, BoardPath, args[0], f, *jsonOut)
		},
	}
	cmd.Flags().StringVar(&state.title, "title", "", "new title")
	cmd.Flags().StringVar(&state.description, "description", "", "new description (pass \"\" to clear)")
	cmd.Flags().StringVar(&state.priority, "priority", "", "new priority (pass \"\" to clear)")
	cmd.Flags().StringVar(&state.tags, "tags", "", "comma-separated replacement tag list")
	cmd.Flags().StringVar(&state.column, "column", "", "move the card to this column (re-orders to bottom)")
	return cmd
}

// runEdit is the testable run body for `ezida edit`.
func runEdit(cmd *cobra.Command, path, id string, f editFlags, asJSON bool) error {
	if !f.any() {
		return &NothingToEditError{}
	}

	card, err := mutateAndSave(path, func(b *board.Board) (board.Card, error) {
		idx := indexCardByID(b.Cards, id)
		if idx < 0 {
			return board.Card{}, &CardNotFoundError{ID: id}
		}
		c := b.Cards[idx]

		if f.title != nil {
			if strings.TrimSpace(*f.title) == "" {
				return board.Card{}, &MissingTitleError{}
			}
			c.Title = *f.title
		}
		if f.description != nil {
			c.Description = *f.description
		}
		if f.priority != nil {
			if *f.priority != "" && !slices.Contains(b.Board.Priorities, *f.priority) {
				return board.Card{}, &InvalidPriorityError{Name: *f.priority}
			}
			c.Priority = *f.priority
		}
		if f.tags != nil {
			tags, terr := parseTags(*f.tags)
			if terr != nil {
				return board.Card{}, terr
			}
			c.Tags = tags
		}
		c.UpdatedAt = nowFunc().UTC().Truncate(time.Second)

		if f.column != nil {
			if !slices.Contains(b.Board.Columns, *f.column) {
				return board.Card{}, &ColumnNotFoundError{Name: *f.column}
			}
			c.Column = *f.column
			b.Cards = slices.Delete(b.Cards, idx, idx+1)
			board.AppendCardToColumn(b, c)
		} else {
			b.Cards[idx] = c
		}
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
