package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// newDummyMigrateForPath builds a migrate command bound to path.
func newDummyMigrateForPath(path string, asJSON bool) *cobra.Command {
	return &cobra.Command{
		Use:  "migrate",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate(cmd, path, asJSON)
		},
	}
}

// TestMigrate_UpgradesAV1Board asserts the whole contract at once: the
// version moves, every card field survives byte-for-byte, and the
// backup holds the pre-migration bytes.
func TestMigrate_UpgradesAV1Board(t *testing.T) {
	path := copyNamedFixture(t, "v1_board.toml")
	original := readFile(t, path)

	before, err := board.LoadUnchecked(path)
	if err != nil {
		t.Fatalf("read v1: %v", err)
	}

	stdout, _, err := executeCobraText(newDummyMigrateForPath(path, false), nil, false)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	after, err := board.Load(path)
	if err != nil {
		t.Fatalf("the migrated board does not load: %v", err)
	}
	if after.SchemaVersion != 2 {
		t.Fatalf("schema_version = %d, want 2", after.SchemaVersion)
	}
	if len(after.Cards) != len(before.Cards) {
		t.Fatalf("card count changed: %d → %d", len(before.Cards), len(after.Cards))
	}
	for i, want := range before.Cards {
		got := after.Cards[i]
		switch {
		case got.ID != want.ID, got.Title != want.Title, got.Column != want.Column,
			got.Description != want.Description, got.Priority != want.Priority,
			!got.CreatedAt.Equal(want.CreatedAt), !got.UpdatedAt.Equal(want.UpdatedAt),
			!equalStrings(got.Tags, want.Tags):
			t.Fatalf("card %s changed:\n got %+v\nwant %+v", want.ID, got, want)
		}
		if got.Epic != "" || got.Color != "" {
			t.Errorf("card %s gained epic/color data", got.ID)
		}
	}
	// Priority colors survive too.
	if after.Board.PriorityColors["high"] != "#ef4444" {
		t.Errorf("priority_colors changed: %v", after.Board.PriorityColors)
	}

	backup := path + ".v1.bak"
	if got := readFile(t, backup); got != original {
		t.Errorf("backup is not byte-identical to the pre-migration file")
	}
	// The report names the source and target versions, the chosen
	// column, and the skill refresh.
	for _, want := range []string{"1", "2", "done", "--skill-only"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("report does not mention %q:\n%s", want, stdout)
		}
	}
}

func TestMigrate_PrefersAColumnNamedDone(t *testing.T) {
	path := copyNamedFixture(t, "v1_board.toml") // backlog, done, archive
	if _, _, err := executeCobraText(newDummyMigrateForPath(path, false), nil, false); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	b := loadBoard(t, path)
	if !b.Board.IsDoneColumn("done") {
		t.Error("done was not marked terminal")
	}
	if b.Board.IsDoneColumn("archive") {
		t.Error("archive was marked terminal")
	}
}

func TestMigrate_FallsBackToTheLastColumn(t *testing.T) {
	path := writeTempV1(t, `schema_version = 1

[board]
columns = ["todo", "wip", "shipped"]
priorities = ["low"]
`)
	if _, _, err := executeCobraText(newDummyMigrateForPath(path, false), nil, false); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	b := loadBoard(t, path)
	if !b.Board.IsDoneColumn("shipped") {
		t.Errorf("terminal columns = %v, want [shipped]", b.Board.DoneColumns())
	}
}

func TestMigrate_JSONReportsTheChoice(t *testing.T) {
	path := copyNamedFixture(t, "v1_board.toml")
	stdout, _, err := executeCobraText(newDummyMigrateForPath(path, true), nil, true)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var env struct {
		Migrated    bool   `json:"migrated"`
		FromVersion int    `json:"from_version"`
		ToVersion   int    `json:"to_version"`
		DoneColumn  string `json:"done_column"`
		BackupPath  string `json:"backup_path"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, stdout)
	}
	if !env.Migrated || env.FromVersion != 1 || env.ToVersion != 2 || env.DoneColumn != "done" {
		t.Fatalf("envelope = %+v", env)
	}
	if !strings.HasSuffix(env.BackupPath, ".v1.bak") {
		t.Errorf("backup_path = %q", env.BackupPath)
	}
}

func TestMigrate_RefusesAnAlreadyCurrentBoard(t *testing.T) {
	path := copyEpicsFixture(t)
	before := readFile(t, path)
	_, _, err := executeCobraText(newDummyMigrateForPath(path, false), nil, false)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if got := AsDetailed(err).Code(); got != "MIGRATION_NOT_NEEDED" {
		t.Errorf("code = %s, want MIGRATION_NOT_NEEDED", got)
	}
	if readFile(t, path) != before {
		t.Error("kanban.toml was modified")
	}
}

func TestMigrate_RefusesAFutureVersion(t *testing.T) {
	path := writeTempV1(t, `schema_version = 3

[board]
columns = ["todo"]
priorities = ["low"]
`)
	before := readFile(t, path)
	_, _, err := executeCobraText(newDummyMigrateForPath(path, false), nil, false)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if code, _ := output.Classify(err); code != "SCHEMA_VERSION_MISMATCH" {
		t.Errorf("code = %s, want SCHEMA_VERSION_MISMATCH", code)
	}
	if readFile(t, path) != before {
		t.Error("kanban.toml was modified")
	}
}

// A board that fails validation is refused before anything is written —
// including the backup, so no stray file is left behind.
func TestMigrate_RefusesAnInvalidBoard(t *testing.T) {
	path := writeTempV1(t, `schema_version = 1

[board]
columns = ["todo"]
priorities = ["low"]

[[cards]]
id = "a3f2k9"
title = "Orphaned column"
column = "ghost"
description = ""
created_at = 2026-05-20T14:30:00Z
updated_at = 2026-05-20T14:30:00Z
tags = []
`)
	before := readFile(t, path)
	_, _, err := executeCobraText(newDummyMigrateForPath(path, false), nil, false)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if code, _ := output.Classify(err); code != "VALIDATION_FAILED" {
		t.Errorf("code = %s, want VALIDATION_FAILED", code)
	}
	if readFile(t, path) != before {
		t.Error("kanban.toml was modified")
	}
	if _, err := os.Stat(path + ".v1.bak"); err == nil {
		t.Error("a backup was written for a board that never migrated")
	}
}

// writeTempV1 writes an arbitrary board body to a temp kanban.toml.
func writeTempV1(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kanban.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}
