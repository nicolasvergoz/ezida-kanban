package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// doneMarker is the text-mode indicator for a terminal column. The
// on-disk `*` suffix never appears in output.
const doneMarker = "✓"

// columnsGroup builds the columns-specific refGroup for board b.
func columnsGroup(b *board.Board) *refGroup {
	return &refGroup{
		list:          &b.Board.Columns,
		cardField:     func(c *board.Card) *string { return &c.Column },
		isReferencing: func(v, n string) bool { return v == n },
		inUseErr: func(name string, cards []affectedCard) error {
			return &ColumnInUseError{Name: name, Cards: cards}
		},
		lastErr:      func(name string) error { return &LastColumnError{Name: name} },
		duplicateErr: func(name string) error { return &DuplicateError{Kind: "column", Name: name} },
		unknownErr:   func(name string) error { return &ColumnNotFoundError{Name: name} },
		positionErr:  func(p, m int) error { return &PositionOutOfRangeError{Position: p, Max: m} },
	}
}

// NewColumnsCmd builds the `ezida columns` parent and its
// subcommands. With no subcommand it lists the board's columns.
func NewColumnsCmd(jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "columns",
		Short: "List the board's columns, or manage them with a subcommand",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsList(cmd, BoardPath, *jsonOut)
		},
	}
	cmd.AddCommand(newColumnsAddCmd(jsonOut))
	cmd.AddCommand(newColumnsRenameCmd(jsonOut))
	cmd.AddCommand(newColumnsRmCmd(jsonOut))
	cmd.AddCommand(newColumnsDoneCmd(jsonOut, true))
	cmd.AddCommand(newColumnsDoneCmd(jsonOut, false))
	return cmd
}

func newColumnsDoneCmd(jsonOut *bool, done bool) *cobra.Command {
	use, short := "done <name>", "Mark a column terminal (its cards count as done)"
	if !done {
		use, short = "undone <name>", "Clear a column's terminal marker"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsDone(cmd, BoardPath, args[0], done, *jsonOut)
		},
	}
}

// runColumnsDone marks or clears the terminal flag on a column. Both
// directions are idempotent: when the flag already holds the requested
// value the board is not saved at all, so kanban.toml stays
// byte-unchanged.
//
// The argument is always a bare name. A caller passing `done*` simply
// fails the lookup with COLUMN_NOT_FOUND — rule 16 guarantees no real
// column can carry the marker in its name.
func runColumnsDone(cmd *cobra.Command, path, name string, done, asJSON bool) error {
	b, err := board.Load(path)
	if err != nil {
		return err
	}
	if !slices.Contains(b.Board.Columns, name) {
		return &ColumnNotFoundError{Name: name}
	}
	if b.Board.IsDoneColumn(name) != done {
		b.Board.SetDoneColumn(name, done)
		if err := board.Save(path, b); err != nil {
			return err
		}
	}
	return printConfigChange(cmd, asJSON, name)
}

// runColumnsList prints the board's columns with their card counts and
// a terminal indicator. The `*` suffix is a file-format detail and
// never leaks into the output — text mode uses a checkmark, JSON mode a
// boolean.
func runColumnsList(cmd *cobra.Command, path string, asJSON bool) error {
	b, err := board.Load(path)
	if err != nil {
		return err
	}
	counts := make(map[string]int, len(b.Board.Columns))
	for _, c := range b.Cards {
		counts[c.Column]++
	}

	out := cmd.OutOrStdout()
	if asJSON {
		cols := make([]output.ColumnSummary, 0, len(b.Board.Columns))
		for _, name := range b.Board.Columns {
			cols = append(cols, output.ColumnSummary{
				Name:  name,
				Count: counts[name],
				Done:  b.Board.IsDoneColumn(name),
			})
		}
		buf, err := output.Columns(output.ColumnsEnvelope{Columns: cols})
		if err != nil {
			return err
		}
		_, err = out.Write(buf)
		return err
	}

	rows := make([][]string, 0, len(b.Board.Columns))
	for _, name := range b.Board.Columns {
		marker := ""
		if b.Board.IsDoneColumn(name) {
			marker = doneMarker
		}
		rows = append(rows, []string{name, fmt.Sprintf("%d", counts[name]), marker})
	}
	_, err = out.Write([]byte(output.Table(rows, []string{"COLUMN", "CARDS", "DONE"})))
	return err
}

func newColumnsAddCmd(jsonOut *bool) *cobra.Command {
	var position int
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new column",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsAdd(cmd, BoardPath, args[0], position, *jsonOut)
		},
	}
	cmd.Flags().IntVar(&position, "position", 0, "1-indexed insertion position (default: append)")
	return cmd
}

func newColumnsRenameCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a column (propagates to every referencing card)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsRename(cmd, BoardPath, args[0], args[1], *jsonOut)
		},
	}
}

func newColumnsRmCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a column (refuses when in use)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsRm(cmd, BoardPath, args[0], *jsonOut)
		},
	}
}

func runColumnsAdd(cmd *cobra.Command, path, name string, position int, asJSON bool) error {
	if err := rejectMarker(name); err != nil {
		return err
	}
	if err := mutateBoardAndSave(path, func(b *board.Board) error {
		return columnsGroup(b).add(name, position)
	}); err != nil {
		return err
	}
	return printConfigChange(cmd, asJSON, name)
}

func runColumnsRename(cmd *cobra.Command, path, oldName, newName string, asJSON bool) error {
	if err := rejectMarker(newName); err != nil {
		return err
	}
	if err := mutateBoardAndSave(path, func(b *board.Board) error {
		// refGroup is shared with priorities and knows nothing about
		// terminal columns, so the flag is carried across the rename
		// here rather than inside it.
		wasDone := b.Board.IsDoneColumn(oldName)
		if err := columnsGroup(b).rename(b, oldName, newName); err != nil {
			return err
		}
		if wasDone {
			b.Board.SetDoneColumn(oldName, false)
			b.Board.SetDoneColumn(newName, true)
		}
		return nil
	}); err != nil {
		return err
	}
	return printConfigChange(cmd, asJSON, newName)
}

// rejectMarker refuses a column name carrying the terminal marker.
// The suffix is a serialization detail; accepting it as input would let
// a user write a name the loader cannot round-trip.
func rejectMarker(name string) error {
	if strings.Contains(name, board.TerminalMarker) {
		return &InvalidColumnNameError{Name: name}
	}
	return nil
}

func runColumnsRm(cmd *cobra.Command, path, name string, asJSON bool) error {
	if err := mutateBoardAndSave(path, func(b *board.Board) error {
		return columnsGroup(b).remove(b, name)
	}); err != nil {
		return err
	}
	return printConfigChange(cmd, asJSON, name)
}

// mutateBoardAndSave is a thin variant of mutateAndSave used by the
// columns and priorities subcommands: the mutator does not produce a
// card to echo, only an error.
func mutateBoardAndSave(path string, mutate func(b *board.Board) error) error {
	b, err := board.Load(path)
	if err != nil {
		return err
	}
	if err := mutate(b); err != nil {
		return err
	}
	return board.Save(path, b)
}

// printConfigChange emits the success line for a columns/priorities
// subcommand. Text mode echoes the affected name; JSON mode emits a
// stable `{"ok":true,"name":"..."}` shape.
func printConfigChange(cmd *cobra.Command, asJSON bool, name string) error {
	out := cmd.OutOrStdout()
	if asJSON {
		_, err := fmt.Fprintf(out, "{\"ok\":true,\"name\":%q}\n", name)
		return err
	}
	_, err := fmt.Fprintln(out, name)
	return err
}
