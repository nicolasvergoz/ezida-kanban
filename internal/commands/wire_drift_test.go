package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// TestExport_DoesNotExposeEpicOrColor pins a deliberate, time-boxed
// inconsistency: `ezida get --json` reports epic data while
// `ezida export` does not.
//
// output.ExportCard and server.cardResponse are parallel structs kept
// in shape-sync by convention, and they move together in the wire
// change. Until then this test exists so the gap is a recorded decision
// rather than something rediscovered as a bug — and so that closing it
// is a deliberate edit here, not a silent one.
func TestExport_DoesNotExposeEpicOrColor(t *testing.T) {
	path := copyEpicsFixture(t)
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
	var env struct {
		Cards []map[string]any `json:"cards"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if len(env.Cards) == 0 {
		t.Fatal("export returned no cards")
	}
	for _, c := range env.Cards {
		for _, key := range []string{"epic", "color"} {
			if _, present := c[key]; present {
				t.Errorf("card %v exposes %q — if the wire change has landed, "+
					"update this test rather than deleting it", c["id"], key)
			}
		}
	}
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
