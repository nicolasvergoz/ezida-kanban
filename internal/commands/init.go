package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// BoardPath is the relative path of the kanban.toml the CLI operates
// on. Hard-coded for v1 (design "Cobra root and command registration").
const BoardPath = "kanban.toml"

var (
	defaultColumns    = []string{"todo", "ongoing", "done"}
	defaultPriorities = []string{"low", "medium", "high"}
)

// NewInitCmd builds the `ezida init` command. jsonOut points at the
// root command's --json flag so the run logic can choose the right
// success envelope.
func NewInitCmd(jsonOut *bool) *cobra.Command {
	var (
		columnsCSV    string
		prioritiesCSV string
		force         bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new kanban.toml in the current directory",
		Long: "Write a fresh kanban.toml with default or supplied " +
			"columns and priorities. Refuses to overwrite an existing " +
			"file unless --force is set.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cols := parseCSVOrDefault(columnsCSV, defaultColumns)
			prios := parseCSVOrDefault(prioritiesCSV, defaultPriorities)
			return runInit(cmd, BoardPath, cols, prios, force, *jsonOut)
		},
	}
	cmd.Flags().StringVar(&columnsCSV, "columns", "",
		`comma-separated column names (default "todo,ongoing,done")`)
	cmd.Flags().StringVar(&prioritiesCSV, "priorities", "",
		`comma-separated priority names (default "low,medium,high")`)
	cmd.Flags().BoolVar(&force, "force", false,
		"overwrite an existing kanban.toml")
	return cmd
}

// runInit is the testable run body. It writes a fresh board to path
// and emits the success envelope to cmd.OutOrStdout().
func runInit(cmd *cobra.Command, path string, cols, prios []string, force, asJSON bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return &AlreadyInitializedError{Path: path}
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	now := nowFunc()
	b := &board.Board{
		SchemaVersion: board.SupportedSchemaVersion,
		Board: board.BoardConfig{
			Columns:    cols,
			Priorities: prios,
		},
		Cards: nil,
	}
	_ = now // reserved for future fields; currently unused.

	if err := board.Save(path, b); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if asJSON {
		fmt.Fprintf(out, "{\"initialized\":true,\"path\":%q}\n", path)
	} else {
		fmt.Fprintln(out, "initialized kanban.toml")
	}
	return nil
}

// parseCSVOrDefault splits csv on commas, trims whitespace, drops
// empty values, and returns dflt when the result is empty.
func parseCSVOrDefault(csv string, dflt []string) []string {
	if strings.TrimSpace(csv) == "" {
		return dflt
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return dflt
	}
	return out
}
