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

// addFlags carries the flags parsed by the `add` command.
type addFlags struct {
	column      string
	priority    string
	tagsCSV     string
	description string
	epic        string
	color       string
}

// NewAddCmd builds the `ezida add` command.
func NewAddCmd(jsonOut *bool) *cobra.Command {
	f := addFlags{}
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Create a new card in the target column",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, BoardPath, args[0], f, *jsonOut)
		},
	}
	cmd.Flags().StringVar(&f.column, "column", "", "target column (required)")
	cmd.Flags().StringVar(&f.priority, "priority", "", "optional priority")
	cmd.Flags().StringVar(&f.tagsCSV, "tags", "", "comma-separated tag list")
	cmd.Flags().StringVar(&f.description, "description", "", "free-form card body")
	cmd.Flags().StringVar(&f.epic, "epic", "", "id of the card this one belongs to")
	cmd.Flags().StringVar(&f.color, "color", "",
		"palette name ("+strings.Join(board.PaletteNames(), ", ")+") or hex value")
	_ = cmd.MarkFlagRequired("column")
	return cmd
}

// parseTags splits csv on commas, trims whitespace around each entry,
// and rejects empty entries with *InvalidTagError. An empty input
// returns an empty slice with no error.
func parseTags(csv string) ([]string, error) {
	if csv == "" {
		return []string{}, nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, &InvalidTagError{Raw: csv}
		}
		out = append(out, p)
	}
	return out, nil
}

// runAdd is the testable run body for `ezida add`.
func runAdd(cmd *cobra.Command, path, title string, f addFlags, asJSON bool) error {
	if strings.TrimSpace(title) == "" {
		return &MissingTitleError{}
	}
	tags, err := parseTags(f.tagsCSV)
	if err != nil {
		return err
	}

	card, err := mutateAndSave(path, func(b *board.Board) (board.Card, error) {
		if !slices.Contains(b.Board.Columns, f.column) {
			return board.Card{}, &ColumnNotFoundError{Name: f.column}
		}
		if f.priority != "" && !slices.Contains(b.Board.Priorities, f.priority) {
			return board.Card{}, &InvalidPriorityError{Name: f.priority}
		}
		// The new card has no id yet, so the self-reference arm of
		// CheckEpicTarget cannot fire — which is correct, a card
		// cannot be created under itself.
		if f.epic != "" {
			if err := board.CheckEpicTarget(b, "", f.epic); err != nil {
				return board.Card{}, asEpicError(err)
			}
		}
		color := ""
		if f.color != "" {
			resolved, cerr := board.ResolveColor(f.color)
			if cerr != nil {
				return board.Card{}, &InvalidColorError{Value: f.color}
			}
			color = resolved
		}
		existing := make([]string, 0, len(b.Cards))
		for _, c := range b.Cards {
			existing = append(existing, c.ID)
		}
		id, err := board.NewUniqueID(existing)
		if err != nil {
			return board.Card{}, err
		}
		now := nowFunc().UTC().Truncate(time.Second)
		c := board.Card{
			ID:          id,
			Title:       title,
			Column:      f.column,
			Description: f.description,
			Tags:        tags,
			Priority:    f.priority,
			Epic:        f.epic,
			Color:       color,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		board.PrependCardToColumn(b, c)
		// The parent becomes an epic the moment it acquires this
		// child, so it gets a color in the same write.
		if f.epic != "" && board.EnsureEpicColor(b, f.epic) {
			for i := range b.Cards {
				if b.Cards[i].ID == f.epic {
					b.Cards[i].UpdatedAt = now
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
	_, err = fmt.Fprintln(out, card.ID)
	return err
}
