package commands

import (
	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// prioritiesGroup builds the priorities-specific refGroup for board b.
func prioritiesGroup(b *board.Board) *refGroup {
	return &refGroup{
		list:          &b.Board.Priorities,
		cardField:     func(c *board.Card) *string { return &c.Priority },
		isReferencing: func(v, n string) bool { return v != "" && v == n },
		inUseErr: func(name string, cards []affectedCard) error {
			return &PriorityInUseError{Name: name, Cards: cards}
		},
		lastErr:      func(name string) error { return &LastPriorityError{Name: name} },
		duplicateErr: func(name string) error { return &DuplicateError{Kind: "priority", Name: name} },
		unknownErr:   func(name string) error { return &InvalidPriorityError{Name: name} },
		positionErr:  func(p, m int) error { return &PositionOutOfRangeError{Position: p, Max: m} },
	}
}

// NewPrioritiesCmd builds the `ezida priorities` parent and its three
// subcommands. Unlike columns, `add` does NOT expose `--position`.
func NewPrioritiesCmd(jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "priorities",
		Short: "Manage the board's priorities",
	}
	cmd.AddCommand(newPrioritiesAddCmd(jsonOut))
	cmd.AddCommand(newPrioritiesRenameCmd(jsonOut))
	cmd.AddCommand(newPrioritiesRmCmd(jsonOut))
	return cmd
}

func newPrioritiesAddCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new priority (appended to the end)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrioritiesAdd(cmd, BoardPath, args[0], *jsonOut)
		},
	}
}

func newPrioritiesRenameCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a priority (propagates to every referencing card)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrioritiesRename(cmd, BoardPath, args[0], args[1], *jsonOut)
		},
	}
}

func newPrioritiesRmCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a priority (refuses when in use)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrioritiesRm(cmd, BoardPath, args[0], *jsonOut)
		},
	}
}

func runPrioritiesAdd(cmd *cobra.Command, path, name string, asJSON bool) error {
	if err := mutateBoardAndSave(path, func(b *board.Board) error {
		// position=0 → append at end (priorities have no --position).
		return prioritiesGroup(b).add(name, 0)
	}); err != nil {
		return err
	}
	return printConfigChange(cmd, asJSON, name)
}

func runPrioritiesRename(cmd *cobra.Command, path, oldName, newName string, asJSON bool) error {
	if err := mutateBoardAndSave(path, func(b *board.Board) error {
		return prioritiesGroup(b).rename(b, oldName, newName)
	}); err != nil {
		return err
	}
	return printConfigChange(cmd, asJSON, newName)
}

func runPrioritiesRm(cmd *cobra.Command, path, name string, asJSON bool) error {
	if err := mutateBoardAndSave(path, func(b *board.Board) error {
		return prioritiesGroup(b).remove(b, name)
	}); err != nil {
		return err
	}
	return printConfigChange(cmd, asJSON, name)
}
