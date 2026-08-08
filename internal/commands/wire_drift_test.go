package commands

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// TestExport_ExposesEpicAndColor closes the deliberate, time-boxed
// inconsistency this file used to pin: `ezida get --json` reported epic
// data while `ezida export` did not.
//
// output.ExportCard and server.cardResponse are parallel structs kept
// in shape-sync by convention; the structural half of that contract is
// enforced by TestWireShape_ExportMatchesBoard in internal/server. This
// test covers the other half — that runExport actually populates the
// fields it declares.
func TestExport_ExposesEpicAndColor(t *testing.T) {
	env := exportFixture(t, copyEpicsFixture(t))
	byID := make(map[string]map[string]any, len(env.Cards))
	for _, c := range env.Cards {
		byID[c["id"].(string)] = c
	}
	if len(byID) == 0 {
		t.Fatal("export returned no cards")
	}
	if got := byID["f20wbo"]["epic"]; got != "rl4m9x" {
		t.Errorf("f20wbo.epic = %v, want rl4m9x", got)
	}
	if got := byID["rl4m9x"]["color"]; got != "#8b5cf6" {
		t.Errorf("rl4m9x.color = %v, want #8b5cf6", got)
	}
	// Like /api/board, the export carries the raw ids only — resolving
	// the relation is the consumer's job (design D1).
	for _, c := range env.Cards {
		for _, key := range []string{"epic_title", "epic_color", "children", "progress"} {
			if _, present := c[key]; present {
				t.Errorf("card %v carries denormalized field %q", c["id"], key)
			}
		}
	}
}

// TestExport_TerminalColumnsAreBarePlusDoneColumns checks that the
// `*` spelling stays on disk and terminal status travels in its own
// array, exactly as GET /api/board reports it.
func TestExport_TerminalColumnsAreBarePlusDoneColumns(t *testing.T) {
	env := exportFixture(t, copyEpicsFixture(t))
	wantCols := []string{"backlog", "todo", "done"}
	if !reflect.DeepEqual(env.Columns, wantCols) {
		t.Errorf("columns = %v, want %v", env.Columns, wantCols)
	}
	wantDone := []string{"done"}
	if !reflect.DeepEqual(env.DoneColumns, wantDone) {
		t.Errorf("done_columns = %v, want %v", env.DoneColumns, wantDone)
	}
	if strings.Contains(env.raw, "*") {
		t.Errorf("export output contains a '*' character: %s", env.raw)
	}
}

// exportEnvelope is the decoded export output plus the raw bytes, so
// tests can assert both on structure and on key presence (omitempty is
// only observable through the generic card maps).
type exportEnvelope struct {
	Columns     []string         `json:"columns"`
	DoneColumns []string         `json:"done_columns"`
	Cards       []map[string]any `json:"cards"`
	raw         string
}

// exportFixture runs `ezida export` against the board at path and
// returns the decoded envelope.
func exportFixture(t *testing.T, path string) exportEnvelope {
	t.Helper()
	cmd := &cobra.Command{
		Use: "export", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(cmd, path, true)
		},
	}
	stdout, _, err := executeCobraText(cmd, nil, true)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	var env exportEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	env.raw = stdout
	return env
}

// The two directions of a version mismatch have different answers, and
// confusing them sends the user down a path that cannot work.
func TestSchemaVersionMismatch_NamesTheRightRemedy(t *testing.T) {
	cases := []struct {
		name         string
		err          error
		wantMigrate  bool
		wantContains string
	}{
		{
			name:         "older file",
			err:          &board.SchemaVersionError{FileVersion: 1, SupportedVersion: 2},
			wantMigrate:  true,
			wantContains: "ezida migrate",
		},
		{
			name:         "newer file",
			err:          &board.SchemaVersionError{FileVersion: 3, SupportedVersion: 2},
			wantMigrate:  false,
			wantContains: "upgrade ezida",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr := &strings.Builder{}
			exit := output.FailTo(stderr, tc.err, false)
			if exit != 1 {
				t.Errorf("exit = %d, want 1", exit)
			}
			msg := stderr.String()
			if !strings.Contains(msg, tc.wantContains) {
				t.Errorf("message %q does not contain %q", msg, tc.wantContains)
			}
			if got := strings.Contains(msg, "ezida migrate"); got != tc.wantMigrate {
				t.Errorf("mentions `ezida migrate` = %v, want %v: %q", got, tc.wantMigrate, msg)
			}
			if code, _ := output.Classify(tc.err); code != "SCHEMA_VERSION_MISMATCH" {
				t.Errorf("code = %s", code)
			}
		})
	}
}
