package commands

import (
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// listFlags carries the filter flags parsed from the command line.
type listFlags struct {
	column          string
	titleContains   string
	tag             string
	priority        string
	epic            string
	includeArchived bool
	archivedOnly    bool
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
	registerListFilterFlags(cmd, &f)
	cmd.Flags().BoolVar(&f.includeArchived, "include-archived", false,
		"append archived cards to the results")
	cmd.Flags().BoolVar(&f.archivedOnly, "archived-only", false,
		"return archived cards instead of live ones")
	return cmd
}

// newArchiveListCmd builds `ezida archive list`, a thin wrapper over
// runList with archivedOnly pinned — one implementation, one envelope.
func newArchiveListCmd(jsonOut *bool) *cobra.Command {
	f := listFlags{archivedOnly: true}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List archived cards, optionally filtered",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, BoardPath, f, *jsonOut)
		},
	}
	registerListFilterFlags(cmd, &f)
	return cmd
}

func registerListFilterFlags(cmd *cobra.Command, f *listFlags) {
	cmd.Flags().StringVar(&f.column, "column", "", "keep only cards in this column")
	cmd.Flags().StringVar(&f.titleContains, "title-contains", "",
		"keep only cards whose title contains this substring (case-insensitive)")
	cmd.Flags().StringVar(&f.tag, "tag", "", "keep only cards carrying this tag")
	cmd.Flags().StringVar(&f.priority, "priority", "", "keep only cards with this priority")
	cmd.Flags().StringVar(&f.epic, "epic", "", "keep only this card and the cards belonging to it")
}

// filter is a card predicate. It is applied identically to live cards
// and to the embedded board.Card of an archived one.
type filter func(board.Card) bool

// buildFilters turns the flag values into AND-combined predicates.
// Unknown column / priority values produce a typed *InvalidFilterError
// so the CLI exits with INVALID_FILTER (spec) — except under
// archivedOnly, where a value present only among archived cards is
// also accepted, since an archived card can reference a column,
// priority or epic the live board no longer declares.
func buildFilters(f listFlags, b *board.Board, archive *board.Archive) ([]filter, error) {
	var fs []filter
	if f.column != "" {
		if !slices.Contains(b.Board.Columns, f.column) && !(f.archivedOnly && archiveHasColumn(archive, f.column)) {
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
		if !slices.Contains(b.Board.Priorities, f.priority) && !(f.archivedOnly && archiveHasPriority(archive, f.priority)) {
			return nil, &InvalidFilterError{Flag: "priority", Value: f.priority}
		}
		pri := f.priority
		fs = append(fs, func(c board.Card) bool { return c.Priority == pri })
	}
	if f.epic != "" {
		if indexCardByID(b.Cards, f.epic) < 0 && !(f.archivedOnly && archiveHasID(archive, f.epic)) {
			return nil, &InvalidFilterError{Flag: "epic", Value: f.epic}
		}
		epic := f.epic
		// The parent is kept alongside its children: scoping to an
		// epic must never hide the epic itself.
		fs = append(fs, func(c board.Card) bool {
			return c.ID == epic || c.Epic == epic
		})
	}
	return fs, nil
}

func archiveHasColumn(a *board.Archive, column string) bool {
	if a == nil {
		return false
	}
	for _, c := range a.Cards {
		if c.Column == column {
			return true
		}
	}
	return false
}

func archiveHasPriority(a *board.Archive, priority string) bool {
	if a == nil {
		return false
	}
	for _, c := range a.Cards {
		if c.Priority == priority {
			return true
		}
	}
	return false
}

func archiveHasID(a *board.Archive, id string) bool {
	if a == nil {
		return false
	}
	for _, c := range a.Cards {
		if c.ID == id {
			return true
		}
	}
	return false
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

// applyArchivedFilters is applyFilters' counterpart for archived cards,
// evaluating each predicate against the embedded board.Card.
func applyArchivedFilters(cards []board.ArchivedCard, fs []filter) []board.ArchivedCard {
	if len(fs) == 0 {
		return cards
	}
	out := make([]board.ArchivedCard, 0, len(cards))
cardLoop:
	for _, c := range cards {
		for _, p := range fs {
			if !p(c.Card) {
				continue cardLoop
			}
		}
		out = append(out, c)
	}
	return out
}

// runList is the testable run body for `ezida list` and `ezida archive
// list`.
func runList(cmd *cobra.Command, path string, f listFlags, asJSON bool) error {
	if f.includeArchived && f.archivedOnly {
		return &MutuallyExclusiveFlagsError{Flags: []string{"--include-archived", "--archived-only"}}
	}

	b, err := board.Load(path)
	if err != nil {
		return err
	}

	var archive *board.Archive
	if f.includeArchived || f.archivedOnly {
		archive, err = loadArchive(board.ArchivePathFor(path))
		if err != nil {
			return err
		}
	}

	fs, err := buildFilters(f, b, archive)
	if err != nil {
		return err
	}

	var liveKept []board.Card
	if !f.archivedOnly {
		liveKept = applyFilters(b.Cards, fs)
	}
	var archivedKept []board.ArchivedCard
	if f.includeArchived || f.archivedOnly {
		archivedKept = applyArchivedFilters(archive.Cards, fs)
	}

	out := cmd.OutOrStdout()
	showArchivedColumn := f.includeArchived || f.archivedOnly

	if asJSON {
		lc := make([]output.ListCard, 0, len(liveKept)+len(archivedKept))
		for _, c := range liveKept {
			lc = append(lc, listCardOf(c, nil))
		}
		for _, c := range archivedKept {
			at := c.ArchivedAt
			lc = append(lc, listCardOf(c.Card, &at))
		}
		buf, err := output.List(output.ListEnvelope{Cards: lc})
		if err != nil {
			return err
		}
		_, err = out.Write(buf)
		return err
	}

	headers := []string{"ID", "COLUMN", "PRI", "TITLE", "TAGS"}
	if showArchivedColumn {
		headers = append(headers, "ARCHIVED")
	}
	rows := make([][]string, 0, len(liveKept)+len(archivedKept))
	for _, c := range liveKept {
		row := listTextRow(c)
		if showArchivedColumn {
			row = append(row, "-")
		}
		rows = append(rows, row)
	}
	for _, c := range archivedKept {
		row := listTextRow(c.Card)
		if showArchivedColumn {
			row = append(row, c.ArchivedAt.Format("2006-01-02"))
		}
		rows = append(rows, row)
	}
	_, err = out.Write([]byte(output.Table(rows, headers)))
	return err
}

func listCardOf(c board.Card, archivedAt *time.Time) output.ListCard {
	tags := c.Tags
	if tags == nil {
		tags = []string{}
	}
	return output.ListCard{
		ID:         c.ID,
		Title:      c.Title,
		Column:     c.Column,
		Priority:   c.Priority,
		Tags:       tags,
		Epic:       c.Epic,
		Color:      c.Color,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
		ArchivedAt: archivedAt,
	}
}

func listTextRow(c board.Card) []string {
	pri := c.Priority
	if pri == "" {
		pri = "-"
	}
	tags := "-"
	if len(c.Tags) > 0 {
		tags = strings.Join(c.Tags, ",")
	}
	return []string{c.ID, c.Column, pri, c.Title, tags}
}
