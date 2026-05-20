package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// newDummyRmForPath wires runRm to the given path with an injectable IO
// so tests can drive the prompt via bytes.Buffer / strings.Reader.
func newDummyRmForPath(path string, asJSON bool, rio rmIO) (*cobra.Command, *rmFlags) {
	f := &rmFlags{}
	cmd := &cobra.Command{
		Use:  "rm",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRm(cmd, path, args[0], *f, asJSON, rio)
		},
	}
	cmd.Flags().BoolVar(&f.yes, "yes", false, "")
	return cmd, f
}

// --- promptConfirm ---

func TestPromptConfirm_Y(t *testing.T) {
	w := &bytes.Buffer{}
	got, err := promptConfirm(w, strings.NewReader("y\n"), "prompt: ")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !got {
		t.Errorf("got false, want true")
	}
	if w.String() != "prompt: " {
		t.Errorf("prompt not written: %q", w.String())
	}
}

func TestPromptConfirm_YUppercase(t *testing.T) {
	got, err := promptConfirm(&bytes.Buffer{}, strings.NewReader("Y\n"), "p")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !got {
		t.Errorf("got false, want true")
	}
}

func TestPromptConfirm_N(t *testing.T) {
	got, _ := promptConfirm(&bytes.Buffer{}, strings.NewReader("n\n"), "p")
	if got {
		t.Errorf("got true, want false")
	}
}

func TestPromptConfirm_Empty(t *testing.T) {
	got, _ := promptConfirm(&bytes.Buffer{}, strings.NewReader("\n"), "p")
	if got {
		t.Errorf("got true, want false")
	}
}

func TestPromptConfirm_Garbage(t *testing.T) {
	got, _ := promptConfirm(&bytes.Buffer{}, strings.NewReader("garbage\n"), "p")
	if got {
		t.Errorf("got true, want false")
	}
}

// --- runRm ---

func TestRm_WithYes(t *testing.T) {
	path := copyFixture(t)
	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: false}
	cmd, f := newDummyRmForPath(path, false, rio)
	f.yes = true
	stdout, _, err := executeCobraText(cmd, []string{"a3f2k9", "--yes"}, false)
	if err != nil {
		t.Fatalf("rm: %v", err)
	}
	if !strings.Contains(stdout, "removed a3f2k9") {
		t.Errorf("stdout: %q", stdout)
	}
	b, _ := board.Load(path)
	for _, c := range b.Cards {
		if c.ID == "a3f2k9" {
			t.Errorf("card still present")
		}
	}
}

func TestRm_InteractiveAccept(t *testing.T) {
	path := copyFixture(t)
	stderr := &bytes.Buffer{}
	rio := rmIO{in: strings.NewReader("y\n"), err: stderr, interactive: true}
	cmd, _ := newDummyRmForPath(path, false, rio)
	stdout, _, err := executeCobraText(cmd, []string{"a3f2k9"}, false)
	if err != nil {
		t.Fatalf("rm: %v", err)
	}
	if !strings.Contains(stdout, "removed a3f2k9") {
		t.Errorf("stdout: %q", stdout)
	}
	prompt := stderr.String()
	if !strings.HasPrefix(prompt, `Delete card a3f2k9 "`) {
		t.Errorf("prompt: %q", prompt)
	}
	if !strings.Contains(prompt, `? [y/N] `) {
		t.Errorf("prompt missing [y/N] tail: %q", prompt)
	}
	b, _ := board.Load(path)
	for _, c := range b.Cards {
		if c.ID == "a3f2k9" {
			t.Errorf("card still present")
		}
	}
}

func TestRm_InteractiveReject(t *testing.T) {
	path := copyFixture(t)
	pre, _ := os.ReadFile(path)
	stderr := &bytes.Buffer{}
	rio := rmIO{in: strings.NewReader("n\n"), err: stderr, interactive: true}
	cmd, _ := newDummyRmForPath(path, false, rio)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9"}, false)
	if err != nil {
		t.Fatalf("rm reject: %v", err)
	}
	if !strings.Contains(stderr.String(), "aborted") {
		t.Errorf("stderr missing 'aborted': %q", stderr.String())
	}
	post, _ := os.ReadFile(path)
	if !bytes.Equal(pre, post) {
		t.Errorf("file modified despite rejection")
	}
}

func TestRm_JSONWithoutYesRejects(t *testing.T) {
	path := copyFixture(t)
	pre, _ := os.ReadFile(path)
	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: true}
	cmd, _ := newDummyRmForPath(path, true, rio)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var ire *InteractiveRequiredError
	if !errors.As(err, &ire) {
		t.Errorf("got %T, want *InteractiveRequiredError", err)
	}
	post, _ := os.ReadFile(path)
	if !bytes.Equal(pre, post) {
		t.Errorf("file modified despite refusal")
	}
}

func TestRm_NonTTYWithoutYesRejects(t *testing.T) {
	path := copyFixture(t)
	pre, _ := os.ReadFile(path)
	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: false}
	cmd, _ := newDummyRmForPath(path, false, rio)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var ire *InteractiveRequiredError
	if !errors.As(err, &ire) {
		t.Errorf("got %T, want *InteractiveRequiredError", err)
	}
	post, _ := os.ReadFile(path)
	if !bytes.Equal(pre, post) {
		t.Errorf("file modified despite refusal")
	}
}

func TestRm_UnknownCard(t *testing.T) {
	path := copyFixture(t)
	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: false}
	cmd, _ := newDummyRmForPath(path, false, rio)
	_, _, err := executeCobraText(cmd, []string{"zzzzzz", "--yes"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var nfe *CardNotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("got %T, want *CardNotFoundError", err)
	}
}

func TestRm_JSONSuccessEnvelope(t *testing.T) {
	path := copyFixture(t)
	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: false}
	cmd, _ := newDummyRmForPath(path, true, rio)
	stdout, _, err := executeCobraText(cmd, []string{"a3f2k9", "--yes"}, false)
	if err != nil {
		t.Fatalf("rm: %v", err)
	}
	want := "{\"id\":\"a3f2k9\",\"deleted\":true}\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	// Sanity: parsable JSON.
	var raw map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &raw); err != nil {
		t.Errorf("not valid JSON: %v", err)
	}
}

func TestRm_HelpRuns(t *testing.T) {
	jsonFlag := false
	cmd := NewRmCmd(&jsonFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(buf.String(), "--yes") {
		t.Errorf("help missing --yes flag:\n%s", buf.String())
	}
}
