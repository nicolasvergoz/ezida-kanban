package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// NewBoardCmd builds the `ezida board` command. jsonOut points at the
// root command's --json flag.
func NewBoardCmd(jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Show the board's schema, columns, priorities, and per-column counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoard(cmd, BoardPath, *jsonOut)
		},
	}
	return cmd
}

// runBoard is the testable run body for `ezida board`.
func runBoard(cmd *cobra.Command, path string, asJSON bool) error {
	b, err := board.Load(path)
	if err != nil {
		return err
	}

	counts := make(map[string]int, len(b.Board.Columns))
	for _, col := range b.Board.Columns {
		counts[col] = 0
	}
	for _, c := range b.Cards {
		counts[c.Column]++
	}

	out := cmd.OutOrStdout()
	if asJSON {
		env := output.BoardEnvelope{
			SchemaVersion:  b.SchemaVersion,
			Columns:        b.Board.Columns,
			Priorities:     b.Board.Priorities,
			CardsPerColumn: counts,
		}
		buf, err := output.Board(env)
		if err != nil {
			return err
		}
		_, err = out.Write(buf)
		return err
	}

	fmt.Fprintf(out, "schema %d\n", b.SchemaVersion)
	fmt.Fprint(out, "columns:    ")
	for i, col := range b.Board.Columns {
		if i > 0 {
			fmt.Fprint(out, " → ")
		}
		fmt.Fprintf(out, "%s (%d)", col, counts[col])
	}
	fmt.Fprintln(out)
	fmt.Fprint(out, "priorities: ")
	for i, p := range b.Board.Priorities {
		if i > 0 {
			fmt.Fprint(out, " < ")
		}
		fmt.Fprint(out, p)
	}
	fmt.Fprintln(out)
	return nil
}
