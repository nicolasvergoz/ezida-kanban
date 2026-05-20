package commands_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/commands"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// TestIntegration_ErrorCodesSurface walks the full pipeline from a real
// command's RunE → returned error → output.Classify (the same classifier
// that Fail uses) and asserts the stable code matches the spec.
//
// This covers each P3 mutating command's --json failure path. A separate
// FailTo round-trip test in output_test.go already verifies that
// classify's outputs flow into the JSON envelope; here we just confirm
// the chain works end-to-end for every new typed error.
func TestIntegration_ErrorCodesSurface(t *testing.T) {
	dir := t.TempDir()
	_ = filepath.Join(dir, "kanban.toml")

	// Seed a fresh board in dir and a single card via the production
	// command constructors (chdir-ing so the hard-coded BoardPath
	// resolves correctly).
	withinDir(t, dir, func() {
		jsonFlag := false
		initCmd := commands.NewInitCmd(&jsonFlag)
		initCmd.SetOut(&bytes.Buffer{})
		initCmd.SetErr(&bytes.Buffer{})
		initCmd.SetArgs([]string{})
		if err := initCmd.Execute(); err != nil {
			t.Fatalf("seed init: %v", err)
		}
	})

	cases := []struct {
		name     string
		build    func(jsonOut *bool) *cobra.Command
		args     []string
		wantCode string
	}{
		{
			name:     "add unknown column",
			build:    commands.NewAddCmd,
			args:     []string{"Title", "--column=ghost"},
			wantCode: "COLUMN_NOT_FOUND",
		},
		{
			name:     "add unknown priority",
			build:    commands.NewAddCmd,
			args:     []string{"Title", "--column=todo", "--priority=urgent"},
			wantCode: "INVALID_PRIORITY",
		},
		{
			name:     "add empty title",
			build:    commands.NewAddCmd,
			args:     []string{"", "--column=todo"},
			wantCode: "MISSING_TITLE",
		},
		{
			name:     "add invalid tag",
			build:    commands.NewAddCmd,
			args:     []string{"Title", "--column=todo", "--tags=,foo,"},
			wantCode: "INVALID_TAG",
		},
		{
			name:     "move unknown column",
			build:    commands.NewMoveCmd,
			args:     []string{"a3f2k9", "ghost"},
			wantCode: "COLUMN_NOT_FOUND",
		},
		{
			name:     "rm JSON without --yes",
			build:    commands.NewRmCmd,
			args:     []string{"a3f2k9"},
			wantCode: "INTERACTIVE_REQUIRED",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var execErr error
			withinDir(t, dir, func() {
				jsonFlag := true
				cmd := tc.build(&jsonFlag)
				cmd.SetOut(&bytes.Buffer{})
				cmd.SetErr(&bytes.Buffer{})
				cmd.SetArgs(tc.args)
				execErr = cmd.Execute()
			})
			if execErr == nil {
				t.Fatalf("expected error")
			}
			gotCode, _ := output.Classify(execErr)
			if gotCode != tc.wantCode {
				t.Errorf("code: got %q, want %q (err=%v)", gotCode, tc.wantCode, execErr)
			}
			// Sanity: errors.As should resolve the underlying typed CodedError.
			var ce output.CodedError
			if !errors.As(execErr, &ce) {
				t.Errorf("err not a CodedError: %v", execErr)
			}
		})
	}

	_ = strings.TrimSpace // keep import slot if needed later
}

// withinDir runs fn with the working directory set to dir, restoring
// the original directory afterward.
func withinDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() { _ = os.Chdir(old) }()
	fn()
}
