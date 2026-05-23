package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// TestExport_FullEnvelope confirms `ezida export --json` emits every
// key of the boardResponse shape (schema_version, project_name,
// columns, priorities, cards_per_column, cards) with the same types.
func TestExport_FullEnvelope(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "export"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runExport(cmd, path, true); err != nil {
		t.Fatalf("export: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout.String())
	}
	for _, key := range []string{"schema_version", "project_name", "columns", "priorities", "cards_per_column", "cards"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q in envelope: %s", key, stdout.String())
		}
	}
	if got["schema_version"].(float64) != 1 {
		t.Errorf("schema_version = %v, want 1", got["schema_version"])
	}
	cards := got["cards"].([]any)
	if len(cards) != 11 {
		t.Errorf("len(cards) = %d, want 11", len(cards))
	}
	// First card must carry the description field (mirrors viewer
	// /api/board which always includes description, unlike list).
	first := cards[0].(map[string]any)
	if _, ok := first["description"]; !ok {
		t.Errorf("card missing description field: %v", first)
	}
}

// TestExport_ProjectNameFromParentDir asserts the project_name field
// matches the parent-directory basename of the kanban.toml path, the
// same rule used by the viewer server.
func TestExport_ProjectNameFromParentDir(t *testing.T) {
	src, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "my-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(projectDir, "kanban.toml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cmd := &cobra.Command{Use: "export"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runExport(cmd, path, true); err != nil {
		t.Fatalf("export: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["project_name"].(string) != "my-project" {
		t.Errorf("project_name = %q, want %q", got["project_name"], "my-project")
	}
}

// TestExport_EmptyTagsRenderAsArray asserts cards with nil tags emit
// "tags":[] (not null), matching the viewer cardResponse contract.
func TestExport_EmptyTagsRenderAsArray(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "export"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runExport(cmd, path, true); err != nil {
		t.Fatalf("export: %v", err)
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"tags":null`)) {
		t.Errorf("envelope contains tags:null — want tags:[]\n%s", stdout.String())
	}
}

// TestExport_MissingFile asserts the command exits non-zero with an
// fs.ErrNotExist-wrapped error when no kanban.toml is present, mirroring
// the behaviour of the other read commands.
func TestExport_MissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-kanban.toml")
	cmd := &cobra.Command{Use: "export"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := runExport(cmd, missing, true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("got %v, want fs.ErrNotExist-wrapped error", err)
	}
}
