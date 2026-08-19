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

// newDummyArchiveForPath builds an `archive <id>` command that writes
// to the given board/archive path pair.
func newDummyArchiveForPath(boardPath, archivePath string, asJSON bool) *cobra.Command {
	return &cobra.Command{
		Use:  "archive",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArchive(cmd, boardPath, archivePath, args[0], asJSON)
		},
	}
}

// newDummyArchiveColumnForPath mirrors newDummyRmForPath: it wires
// runArchiveColumn to an injectable IO so tests can drive the prompt.
func newDummyArchiveColumnForPath(boardPath, archivePath string, asJSON bool, rio rmIO) (*cobra.Command, *archiveColumnFlags) {
	f := &archiveColumnFlags{}
	cmd := &cobra.Command{
		Use:  "column",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArchiveColumn(cmd, boardPath, archivePath, args[0], *f, asJSON, rio)
		},
	}
	cmd.Flags().BoolVar(&f.yes, "yes", false, "")
	return cmd, f
}

// --- archive <id> ---

func TestArchive_MovesCardOutOfBoard(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	cmd := newDummyArchiveForPath(boardPath, archivePath, false)

	stdout, _, err := executeCobraText(cmd, []string{"a3f2k9"}, false)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if strings.TrimSpace(stdout) != "a3f2k9" {
		t.Fatalf("stdout = %q, want a3f2k9", stdout)
	}

	b, err := board.Load(boardPath)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	for _, c := range b.Cards {
		if c.ID == "a3f2k9" {
			t.Fatal("card still present on the board")
		}
	}

	a, err := loadArchive(archivePath)
	if err != nil {
		t.Fatalf("loadArchive: %v", err)
	}
	found := false
	for _, c := range a.Cards {
		if c.ID == "a3f2k9" {
			found = true
			if c.ArchivedAt.IsZero() {
				t.Fatal("archived card has zero ArchivedAt")
			}
		}
	}
	if !found {
		t.Fatal("card not found in archive")
	}
}

func TestArchive_CascadeReportsChildrenOnStderr(t *testing.T) {
	boardPath := copyEpicsFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	cmd := newDummyArchiveForPath(boardPath, archivePath, false)

	stdout, stderr, err := executeCobraText(cmd, []string{"rl4m9x"}, false)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if strings.TrimSpace(stdout) != "rl4m9x" {
		t.Fatalf("stdout = %q, want rl4m9x only", stdout)
	}
	for _, child := range []string{"f20wbo", "wrshlo", "q7t6z2"} {
		if !strings.Contains(stderr, child) {
			t.Errorf("stderr missing cascaded child %q: %q", child, stderr)
		}
	}
}

func TestArchive_JSONEnvelopeShape(t *testing.T) {
	boardPath := copyEpicsFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	cmd := newDummyArchiveForPath(boardPath, archivePath, true)

	stdout, _, err := executeCobraText(cmd, []string{"rl4m9x"}, true)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	line := strings.TrimSpace(stdout)
	if !strings.HasPrefix(line, `{"id":"rl4m9x","archived":true,"cascaded":[`) {
		t.Fatalf("unexpected key order: %s", line)
	}
	var body struct {
		ID       string   `json:"id"`
		Archived bool     `json:"archived"`
		Cascaded []string `json:"cascaded"`
	}
	if err := json.Unmarshal([]byte(line), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Cascaded) != 3 {
		t.Fatalf("cascaded length = %d, want 3", len(body.Cascaded))
	}
}

func TestArchive_UnknownID_CardNotFound(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	preBoard, _ := os.ReadFile(boardPath)
	cmd := newDummyArchiveForPath(boardPath, archivePath, false)

	_, _, err := executeCobraText(cmd, []string{"zzzzzz"}, false)
	var cnf *CardNotFoundError
	if !errors.As(err, &cnf) {
		t.Fatalf("err = %v, want *CardNotFoundError", err)
	}
	postBoard, _ := os.ReadFile(boardPath)
	if !bytes.Equal(preBoard, postBoard) {
		t.Fatal("board file modified despite CARD_NOT_FOUND")
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatal("archive file created despite CARD_NOT_FOUND")
	}
}

// --- archive column ---

func TestArchiveColumn_LeavesColumnInPlace(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: false}
	cmd, f := newDummyArchiveColumnForPath(boardPath, archivePath, false, rio)
	f.yes = true

	_, _, err := executeCobraText(cmd, []string{"done", "--yes"}, false)
	if err != nil {
		t.Fatalf("archive column: %v", err)
	}
	b, err := board.Load(boardPath)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	found := false
	for _, col := range b.Board.Columns {
		if col == "done" {
			found = true
		}
	}
	if !found {
		t.Fatal("column 'done' removed; archive column must leave it in place")
	}
	for _, c := range b.Cards {
		if c.Column == "done" {
			t.Fatalf("card %q still in column 'done'", c.ID)
		}
	}
}

func TestArchiveColumn_PromptsWhenCascadeLeavesColumn(t *testing.T) {
	// epics.toml: rl4m9x (todo) has children f20wbo (backlog), wrshlo
	// (done), q7t6z2 (backlog) — archiving "todo" cascades into backlog.
	boardPath := copyEpicsFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	preBoard, _ := os.ReadFile(boardPath)
	stderr := &bytes.Buffer{}
	rio := rmIO{in: strings.NewReader("n\n"), err: stderr, interactive: true}
	cmd, _ := newDummyArchiveColumnForPath(boardPath, archivePath, false, rio)

	_, _, err := executeCobraText(cmd, []string{"todo"}, false)
	if err != nil {
		t.Fatalf("archive column: %v", err)
	}
	if !strings.Contains(stderr.String(), "aborted") {
		t.Fatalf("stderr missing 'aborted': %q", stderr.String())
	}
	postBoard, _ := os.ReadFile(boardPath)
	if !bytes.Equal(preBoard, postBoard) {
		t.Fatal("board file modified despite declining the prompt")
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatal("archive file created despite declining the prompt")
	}
}

func TestArchiveColumn_JSONWithoutYes_InteractiveRequired(t *testing.T) {
	boardPath := copyEpicsFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	preBoard, _ := os.ReadFile(boardPath)
	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: true}
	cmd, _ := newDummyArchiveColumnForPath(boardPath, archivePath, true, rio)

	_, _, err := executeCobraText(cmd, []string{"todo"}, false)
	var ire *InteractiveRequiredError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %v, want *InteractiveRequiredError", err)
	}
	postBoard, _ := os.ReadFile(boardPath)
	if !bytes.Equal(preBoard, postBoard) {
		t.Fatal("board file modified despite INTERACTIVE_REQUIRED")
	}
}

func TestArchiveColumn_EmptyColumnWritesNothing(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	// "ongoing" holds exactly one card in the fixture; move it out
	// first isn't necessary — use a genuinely empty column instead by
	// adding one, matching the spec's own scenario shape.
	b, err := board.Load(boardPath)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	if err := board.AddColumn(b, "review"); err != nil {
		t.Fatalf("AddColumn: %v", err)
	}
	if err := board.Save(boardPath, b); err != nil {
		t.Fatalf("board.Save: %v", err)
	}
	preBoard, _ := os.ReadFile(boardPath)

	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: false}
	cmd, f := newDummyArchiveColumnForPath(boardPath, archivePath, false, rio)
	f.yes = true
	stdout, _, err := executeCobraText(cmd, []string{"review", "--yes"}, false)
	if err != nil {
		t.Fatalf("archive column: %v", err)
	}
	if !strings.Contains(stdout, "archived 0 cards") {
		t.Fatalf("stdout = %q, want an 'archived 0 cards' message", stdout)
	}
	postBoard, _ := os.ReadFile(boardPath)
	if !bytes.Equal(preBoard, postBoard) {
		t.Fatal("board file modified by an empty-column archive")
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatal("archive file created by an empty-column archive")
	}
}
