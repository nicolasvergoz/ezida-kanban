package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// seedBoardWithTwoTodoRefs writes a fresh kanban.toml at dir/kanban.toml
// containing two cards that reference the "todo" column.
func seedBoardWithTwoTodoRefs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/kanban.toml"
	at, _ := time.Parse(time.RFC3339, "2026-05-20T14:30:00Z")
	b := &board.Board{
		SchemaVersion: 1,
		Board: board.BoardConfig{
			Columns:    []string{"todo", "ongoing", "done"},
			Priorities: []string{"low", "medium", "high"},
		},
		Cards: []board.Card{
			{ID: "a3f2k9", Title: "Refactor auth", Column: "todo", Tags: []string{}, CreatedAt: at, UpdatedAt: at},
			{ID: "b7m1p4", Title: "Update README", Column: "todo", Tags: []string{}, CreatedAt: at.Add(time.Minute), UpdatedAt: at.Add(time.Minute)},
		},
	}
	if err := board.Save(path, b); err != nil {
		t.Fatalf("save seed: %v", err)
	}
	return path
}

// runColumnsRmForPath builds a thin cobra command that invokes
// runColumnsRm against the given path. Used by the refusal integration
// tests to capture the returned typed error.
func runColumnsRmForPath(t *testing.T, path string, asJSON bool, name string) error {
	t.Helper()
	cmd := &cobra.Command{
		Use:  "rm",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsRm(cmd, path, args[0], asJSON)
		},
	}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{name})
	return cmd.Execute()
}

// TestRefusalPayload_TextRendering covers task 6.2.
func TestRefusalPayload_TextRendering(t *testing.T) {
	path := seedBoardWithTwoTodoRefs(t)
	err := runColumnsRmForPath(t, path, false, "todo")
	if err == nil {
		t.Fatal("expected error")
	}
	var sink bytes.Buffer
	exit := output.FailTo(&sink, err, false)
	if exit != 1 {
		t.Errorf("exit: got %d, want 1", exit)
	}
	got := sink.String()
	for _, want := range []string{
		`Error: column "todo"`,
		"  a3f2k9  Refactor auth",
		"  b7m1p4  Update README",
		"Move or remove these cards first.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestRefusalPayload_JSONRendering covers task 6.3.
func TestRefusalPayload_JSONRendering(t *testing.T) {
	path := seedBoardWithTwoTodoRefs(t)
	err := runColumnsRmForPath(t, path, true, "todo")
	if err == nil {
		t.Fatal("expected error")
	}
	var sink bytes.Buffer
	exit := output.FailTo(&sink, err, true)
	if exit != 1 {
		t.Errorf("exit: got %d, want 1", exit)
	}
	var raw map[string]any
	if err := json.Unmarshal(sink.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, sink.String())
	}
	errObj := raw["error"].(map[string]any)
	if errObj["code"] != "COLUMN_IN_USE" {
		t.Errorf("code: %v", errObj["code"])
	}
	det := errObj["details"].(map[string]any)
	if det["column"] != "todo" {
		t.Errorf("column: %v", det["column"])
	}
	cards := det["cards"].([]any)
	if len(cards) != 2 {
		t.Fatalf("cards: got %d, want 2", len(cards))
	}
	for _, c := range cards {
		obj := c.(map[string]any)
		for _, key := range []string{"id", "title"} {
			if _, has := obj[key]; !has {
				t.Errorf("missing %q in card %v", key, obj)
			}
		}
	}
}
