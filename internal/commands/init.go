package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/skill"
)

// BoardPath is the relative path of the kanban.toml the CLI operates
// on. Hard-coded for v1 (design "Cobra root and command registration").
const BoardPath = "kanban.toml"

// SkillPath is the relative path of the embedded skill file written by
// `ezida init`. Hard-coded for v1 — matches the Claude Code skill
// discovery convention (.claude/skills/<name>/SKILL.md).
var SkillPath = filepath.Join(".claude", "skills", "ezida-kanban", "SKILL.md")

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
		skillOnly     bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new kanban.toml in the current directory",
		Long: "Write a fresh kanban.toml with default or supplied " +
			"columns and priorities, plus the embedded SKILL.md so " +
			"Claude Code can discover the skill. Refuses to overwrite " +
			"an existing kanban.toml unless --force is set. Use " +
			"--skill-only to refresh the skill file without touching " +
			"the board.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cols := parseCSVOrDefault(columnsCSV, defaultColumns)
			prios := parseCSVOrDefault(prioritiesCSV, defaultPriorities)
			return runInit(cmd, BoardPath, SkillPath, cols, prios, force, skillOnly, *jsonOut)
		},
	}
	cmd.Flags().StringVar(&columnsCSV, "columns", "",
		`comma-separated column names (default "todo,ongoing,done")`)
	cmd.Flags().StringVar(&prioritiesCSV, "priorities", "",
		`comma-separated priority names (default "low,medium,high")`)
	cmd.Flags().BoolVar(&force, "force", false,
		"overwrite an existing kanban.toml")
	cmd.Flags().BoolVar(&skillOnly, "skill-only", false,
		"write only .claude/skills/ezida-kanban/SKILL.md; leave kanban.toml untouched")
	return cmd
}

// runInit is the testable run body. It writes a fresh board to
// boardPath (unless skillOnly is set) and always writes the embedded
// skill bytes to skillPath. The success envelope is emitted to
// cmd.OutOrStdout(); in text mode the trailing TOML-comment note is
// appended on full init only.
func runInit(cmd *cobra.Command, boardPath, skillPath string, cols, prios []string, force, skillOnly, asJSON bool) error {
	out := cmd.OutOrStdout()

	if skillOnly {
		if err := writeSkillFile(skillPath); err != nil {
			return err
		}
		if asJSON {
			fmt.Fprintf(out, "{\"skill_only\":true,\"skill_path\":%q}\n", skillPath)
		} else {
			fmt.Fprintf(out, "wrote %s\n", skillPath)
		}
		return nil
	}

	if _, err := os.Stat(boardPath); err == nil && !force {
		return &AlreadyInitializedError{Path: boardPath}
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

	if err := board.Save(boardPath, b); err != nil {
		return err
	}
	if err := writeSkillFile(skillPath); err != nil {
		return err
	}

	if asJSON {
		fmt.Fprintf(out, "{\"initialized\":true,\"path\":%q,\"skill_path\":%q}\n", boardPath, skillPath)
	} else {
		fmt.Fprintf(out, "initialized %s\n", boardPath)
		fmt.Fprintf(out, "wrote %s\n", skillPath)
		fmt.Fprintln(out, "note: TOML comments are not preserved across ezida writes")
	}
	return nil
}

// writeSkillFile writes the embedded skill bytes to path, creating any
// missing parent directories with mode 0755 and the file with mode
// 0644. Overwrites silently per ADR §D15.
func writeSkillFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, skill.Bytes, 0o644)
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
