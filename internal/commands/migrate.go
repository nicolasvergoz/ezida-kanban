package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// migratableVersion is the only source version `ezida migrate` knows
// how to upgrade from. A file at any other lower version (including a
// file with no schema_version at all, which decodes to 0) is not a
// board this binary can reason about.
const migratableVersion = 1

// skillReminder is emitted on success: the skill file ships inside the
// binary and teaches Claude the CLI surface, so a board upgraded
// without refreshing it leaves Claude writing v1-shaped commands.
const skillReminder = "run `ezida init --skill-only` to refresh the embedded skill"

// NewMigrateCmd builds the `ezida migrate` command.
func NewMigrateCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Upgrade kanban.toml to the schema version this binary supports",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate(cmd, BoardPath, *jsonOut)
		},
	}
}

// runMigrate upgrades a v1 board to v2.
//
// Order matters: the upgraded board is validated in memory first, then
// the original bytes are backed up, and only then is the new file
// written. A board that fails validation therefore leaves both
// kanban.toml and the filesystem untouched.
func runMigrate(cmd *cobra.Command, path string, asJSON bool) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	b, err := board.LoadUnchecked(path)
	if err != nil {
		return err
	}

	from := b.SchemaVersion
	switch {
	case from == board.SupportedSchemaVersion:
		return &MigrationNotNeededError{Version: from}
	case from != migratableVersion:
		return &board.SchemaVersionError{
			FileVersion:      from,
			SupportedVersion: board.SupportedSchemaVersion,
		}
	}

	b.SchemaVersion = board.SupportedSchemaVersion
	terminal, reason := chooseTerminalColumn(b)
	if terminal != "" {
		b.Board.SetDoneColumn(terminal, true)
	}

	if verr := board.Validate(b); verr != nil {
		return verr
	}

	backupPath := fmt.Sprintf("%s.v%d.bak", path, from)
	if err := os.WriteFile(backupPath, original, 0o644); err != nil {
		return err
	}
	if err := board.Save(path, b); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if asJSON {
		buf, err := output.Migrate(output.MigrateEnvelope{
			Migrated:      true,
			FromVersion:   from,
			ToVersion:     b.SchemaVersion,
			DoneColumn:    terminal,
			DoneReason:    reason,
			BackupPath:    backupPath,
			SkillReminder: skillReminder,
		})
		if err != nil {
			return err
		}
		_, err = out.Write(buf)
		return err
	}

	fmt.Fprintf(out, "migrated %s from schema %d to %d\n", path, from, b.SchemaVersion)
	fmt.Fprintf(out, "backup written to %s\n", backupPath)
	if terminal != "" {
		fmt.Fprintf(out, "marked column %q as terminal (%s)\n", terminal, reason)
	}
	fmt.Fprintf(out, "next: %s\n", skillReminder)
	return nil
}

// chooseTerminalColumn picks the column whose cards will count as done,
// and explains the choice. A v1 board declares none, so one has to be
// invented — and inventing it silently would reintroduce exactly the
// "wrong numbers, no signal" failure the suffix encoding exists to
// prevent (design D3/D8).
//
// A column already carrying the marker (possible in a hand-edited file)
// suppresses the automatic choice.
func chooseTerminalColumn(b *board.Board) (name, reason string) {
	if existing := b.Board.DoneColumns(); len(existing) > 0 {
		return existing[0], "already marked in the file"
	}
	for _, col := range b.Board.Columns {
		if col == "done" {
			return col, "a column named done was present"
		}
	}
	if n := len(b.Board.Columns); n > 0 {
		return b.Board.Columns[n-1], "no column named done, so the last one was chosen"
	}
	return "", ""
}
