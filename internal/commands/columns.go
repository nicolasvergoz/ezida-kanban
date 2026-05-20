package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

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

// NewColumnsCmd builds the `ezida columns` parent and its three
// subcommands (`add`, `rename`, `rm`).
func NewColumnsCmd(jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "columns",
		Short: "Manage the board's columns",
	}
	cmd.AddCommand(newColumnsAddCmd(jsonOut))
	cmd.AddCommand(newColumnsRenameCmd(jsonOut))
	cmd.AddCommand(newColumnsRmCmd(jsonOut))
	return cmd
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
	if err := mutateBoardAndSave(path, func(b *board.Board) error {
		return columnsGroup(b).add(name, position)
	}); err != nil {
		return err
	}
	return printConfigChange(cmd, asJSON, name)
}

func runColumnsRename(cmd *cobra.Command, path, oldName, newName string, asJSON bool) error {
	if err := mutateBoardAndSave(path, func(b *board.Board) error {
		return columnsGroup(b).rename(b, oldName, newName)
	}); err != nil {
		return err
	}
	return printConfigChange(cmd, asJSON, newName)
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
