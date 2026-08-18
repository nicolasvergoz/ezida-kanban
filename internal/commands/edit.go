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
	epic        *string
	noEpic      bool
	color       *string
	noColor     bool
}

// any reports whether at least one field was passed.
func (f editFlags) any() bool {
	return f.title != nil || f.description != nil || f.priority != nil ||
		f.tags != nil || f.column != nil || f.epic != nil || f.noEpic ||
		f.color != nil || f.noColor
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
	epic        string
	noEpic      bool
	color       string
	noColor     bool
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
	if cmd.Flags().Changed("epic") {
		v := raw.epic
		f.epic = &v
	}
	if cmd.Flags().Changed("color") {
		v := raw.color
		f.color = &v
	}
	f.noEpic = raw.noEpic
	f.noColor = raw.noColor
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
	cmd.Flags().StringVar(&state.column, "column", "", "move the card to this column (re-orders to top)")
	cmd.Flags().StringVar(&state.epic, "epic", "", "id of the card this one belongs to")
	cmd.Flags().BoolVar(&state.noEpic, "no-epic", false, "detach the card from its epic")
	cmd.Flags().StringVar(&state.color, "color", "",
		"palette name ("+strings.Join(board.PaletteNames(), ", ")+") or hex value")
	cmd.Flags().BoolVar(&state.noColor, "no-color", false, "clear the card's color")
	return cmd
}

// runEdit is the testable run body for `ezida edit`.
func runEdit(cmd *cobra.Command, path, id string, f editFlags, asJSON bool) error {
	if !f.any() {
		return &NothingToEditError{}
	}
	// Contradictory flags are refused before the board is even loaded:
	// there is no defensible reading of "set and clear the same field".
	if f.epic != nil && f.noEpic {
		return &InvalidEpicError{
			ID:     *f.epic,
			Reason: "--epic and --no-epic are mutually exclusive",
		}
	}
	if f.color != nil && f.noColor {
		return &InvalidColorError{
			Value:  *f.color,
			Reason: "--color and --no-color are mutually exclusive",
		}
	}

	// Set when the edit also wrote a color onto the parent, so text
	// mode can report the second card the command touched.
	var coloredParent string

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
		if f.epic != nil && *f.epic != "" {
			if eerr := board.CheckEpicTarget(b, id, *f.epic); eerr != nil {
				return board.Card{}, asEpicError(eerr)
			}
			c.Epic = *f.epic
		} else if f.noEpic || (f.epic != nil && *f.epic == "") {
			c.Epic = ""
		}
		if f.color != nil {
			resolved, cerr := board.ResolveColor(*f.color)
			if cerr != nil {
				return board.Card{}, &InvalidColorError{Value: *f.color}
			}
			c.Color = resolved
		} else if f.noColor {
			c.Color = ""
		}
		now := nowFunc().UTC().Truncate(time.Second)
		c.UpdatedAt = now

		if f.column != nil {
			if !slices.Contains(b.Board.Columns, *f.column) {
				return board.Card{}, &ColumnNotFoundError{Name: *f.column}
			}
			c.Column = *f.column
			b.Cards = slices.Delete(b.Cards, idx, idx+1)
			board.PrependCardToColumn(b, c)
		} else {
			b.Cards[idx] = c
		}
		// Acquiring a child turns the parent into an epic, so it gets
		// a color in the same write. Both writes ride one Save, so
		// there is no partial state; the JSON echo still returns only
		// the edited card, and the parent's change is reported in text
		// mode.
		if c.Epic != "" && board.EnsureEpicColor(b, c.Epic) {
			for i := range b.Cards {
				if b.Cards[i].ID == c.Epic {
					b.Cards[i].UpdatedAt = now
					coloredParent = b.Cards[i].ID
					break
				}
			}
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
	if coloredParent != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "assigned a color to epic %s\n", coloredParent)
	}
	_, err = fmt.Fprintln(out, card.ID)
	return err
}
