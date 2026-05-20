package commands

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// listFlags carries the filter flags parsed from the command line.
type listFlags struct {
	column        string
	titleContains string
	tag           string
	priority      string
}

// NewListCmd builds the `ezida list` command.
func NewListCmd(jsonOut *bool) *cobra.Command {
	f := listFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cards, optionally filtered",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, BoardPath, f, *jsonOut)
		},
	}
	cmd.Flags().StringVar(&f.column, "column", "", "keep only cards in this column")
	cmd.Flags().StringVar(&f.titleContains, "title-contains", "",
		"keep only cards whose title contains this substring (case-insensitive)")
	cmd.Flags().StringVar(&f.tag, "tag", "", "keep only cards carrying this tag")
	cmd.Flags().StringVar(&f.priority, "priority", "", "keep only cards with this priority")
	return cmd
}

// filter is a card predicate.
type filter func(board.Card) bool

// buildFilters turns the flag values into AND-combined predicates.
// Unknown column / priority values produce a typed *InvalidFilterError
// so the CLI exits with INVALID_FILTER (spec).
func buildFilters(f listFlags, b *board.Board) ([]filter, error) {
	var fs []filter
	if f.column != "" {
		if !slices.Contains(b.Board.Columns, f.column) {
			return nil, &InvalidFilterError{Flag: "column", Value: f.column}
		}
		col := f.column
		fs = append(fs, func(c board.Card) bool { return c.Column == col })
	}
	if f.titleContains != "" {
		needle := strings.ToLower(f.titleContains)
		fs = append(fs, func(c board.Card) bool {
			return strings.Contains(strings.ToLower(c.Title), needle)
		})
	}
	if f.tag != "" {
		tag := f.tag
		fs = append(fs, func(c board.Card) bool { return slices.Contains(c.Tags, tag) })
	}
	if f.priority != "" {
		if !slices.Contains(b.Board.Priorities, f.priority) {
			return nil, &InvalidFilterError{Flag: "priority", Value: f.priority}
		}
		pri := f.priority
		fs = append(fs, func(c board.Card) bool { return c.Priority == pri })
	}
	return fs, nil
}

// applyFilters keeps cards for which every predicate returns true.
func applyFilters(cards []board.Card, fs []filter) []board.Card {
	if len(fs) == 0 {
		return cards
	}
	out := make([]board.Card, 0, len(cards))
cardLoop:
	for _, c := range cards {
		for _, p := range fs {
			if !p(c) {
				continue cardLoop
			}
		}
		out = append(out, c)
	}
	return out
}

// runList is the testable run body for `ezida list`.
func runList(cmd *cobra.Command, path string, f listFlags, asJSON bool) error {
	b, err := board.Load(path)
	if err != nil {
		return err
	}
	fs, err := buildFilters(f, b)
	if err != nil {
		return err
	}
	kept := applyFilters(b.Cards, fs)

	out := cmd.OutOrStdout()
	if asJSON {
		lc := make([]output.ListCard, 0, len(kept))
		for _, c := range kept {
			tags := c.Tags
			if tags == nil {
				tags = []string{}
			}
			lc = append(lc, output.ListCard{
				ID:        c.ID,
				Title:     c.Title,
				Column:    c.Column,
				Priority:  c.Priority,
				Tags:      tags,
				CreatedAt: c.CreatedAt,
				UpdatedAt: c.UpdatedAt,
			})
		}
		buf, err := output.List(output.ListEnvelope{Cards: lc})
		if err != nil {
			return err
		}
		_, err = out.Write(buf)
		return err
	}

	headers := []string{"ID", "COLUMN", "PRI", "TITLE", "TAGS"}
	rows := make([][]string, 0, len(kept))
	for _, c := range kept {
		pri := c.Priority
		if pri == "" {
			pri = "-"
		}
		tags := "-"
		if len(c.Tags) > 0 {
			tags = strings.Join(c.Tags, ",")
		}
		rows = append(rows, []string{c.ID, c.Column, pri, c.Title, tags})
	}
	_, err = out.Write([]byte(output.Table(rows, headers)))
	return err
}
