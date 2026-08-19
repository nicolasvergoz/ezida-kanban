package commands

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// newDummyUnarchiveForPath builds an `unarchive <id>` command that
// writes to the given board/archive path pair.
func newDummyUnarchiveForPath(boardPath, archivePath string, asJSON bool) (*cobra.Command, *unarchiveFlags) {
	f := &unarchiveFlags{}
	cmd := &cobra.Command{
		Use:  "unarchive",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnarchive(cmd, boardPath, archivePath, args[0], *f, asJSON)
		},
	}
	cmd.Flags().StringVar(&f.column, "column", "", "")
	return cmd, f
}

func TestUnarchive_RestoresCascade(t *testing.T) {
	boardPath := copyEpicsFixture(t)
	archivePath := board.ArchivePathFor(boardPath)

	archiveCmd := newDummyArchiveForPath(boardPath, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"rl4m9x"}, false); err != nil {
		t.Fatalf("setup archive: %v", err)
	}

	cmd, _ := newDummyUnarchiveForPath(boardPath, archivePath, false)
	stdout, _, err := executeCobraText(cmd, []string{"rl4m9x"}, false)
	if err != nil {
		t.Fatalf("unarchive: %v", err)
	}
	if strings.TrimSpace(stdout) != "rl4m9x" {
		t.Fatalf("stdout = %q, want rl4m9x", stdout)
	}

	b, err := board.Load(boardPath)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	for _, id := range []string{"rl4m9x", "f20wbo", "wrshlo", "q7t6z2"} {
		found := false
		for _, c := range b.Cards {
			if c.ID == id {
				found = true
			}
		}
		if !found {
			t.Fatalf("card %q missing from board after unarchive", id)
		}
	}
}

func TestUnarchive_DeletesArchiveFileWhenLastCardLeaves(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)

	archiveCmd := newDummyArchiveForPath(boardPath, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("setup archive: %v", err)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive file missing after setup: %v", err)
	}

	cmd, _ := newDummyUnarchiveForPath(boardPath, archivePath, false)
	if _, _, err := executeCobraText(cmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("unarchive: %v", err)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatal("archive file still present after restoring the only card")
	}
}

func TestUnarchive_RelocatesWhenColumnGone(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)

	archiveCmd := newDummyArchiveForPath(boardPath, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("setup archive: %v", err)
	}

	// Remove the card's original column ("todo") from the board.
	b, err := board.Load(boardPath)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	b.Cards = nil // clear so DeleteColumn's "has cards" guard doesn't fire for other todo cards
	if err := board.DeleteColumn(b, "todo"); err != nil {
		t.Fatalf("DeleteColumn: %v", err)
	}
	if err := board.Save(boardPath, b); err != nil {
		t.Fatalf("board.Save: %v", err)
	}

	cmd, _ := newDummyUnarchiveForPath(boardPath, archivePath, true)
	stdout, _, err := executeCobraText(cmd, []string{"a3f2k9"}, true)
	if err != nil {
		t.Fatalf("unarchive: %v", err)
	}
	var body struct {
		Relocated bool   `json:"relocated"`
		Column    string `json:"column"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !body.Relocated {
		t.Fatal("relocated = false, want true")
	}
	if body.Column != "ongoing" {
		t.Fatalf("column = %q, want ongoing (board's remaining first column)", body.Column)
	}
}

func TestUnarchive_ColumnFlagOverride(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)

	archiveCmd := newDummyArchiveForPath(boardPath, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("setup archive: %v", err)
	}

	cmd, _ := newDummyUnarchiveForPath(boardPath, archivePath, false)
	if _, _, err := executeCobraText(cmd, []string{"a3f2k9", "--column=done"}, false); err != nil {
		t.Fatalf("unarchive: %v", err)
	}
	b, err := board.Load(boardPath)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	for _, c := range b.Cards {
		if c.ID == "a3f2k9" && c.Column != "done" {
			t.Fatalf("column = %q, want done", c.Column)
		}
	}
}

func TestUnarchive_UnknownExplicitColumn(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)

	archiveCmd := newDummyArchiveForPath(boardPath, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("setup archive: %v", err)
	}
	preBoard, _ := os.ReadFile(boardPath)
	preArchive, _ := os.ReadFile(archivePath)

	cmd, _ := newDummyUnarchiveForPath(boardPath, archivePath, false)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9", "--column=ghost"}, false)
	var cnf *ColumnNotFoundError
	if !errors.As(err, &cnf) {
		t.Fatalf("err = %v, want *ColumnNotFoundError", err)
	}
	postBoard, _ := os.ReadFile(boardPath)
	postArchive, _ := os.ReadFile(archivePath)
	if string(preBoard) != string(postBoard) || string(preArchive) != string(postArchive) {
		t.Fatal("files modified despite COLUMN_NOT_FOUND")
	}
}

func TestUnarchive_UnknownID_CardNotArchived(t *testing.T) {
	boardPath := copyFixture(t)
	archivePath := board.ArchivePathFor(boardPath)
	cmd, _ := newDummyUnarchiveForPath(boardPath, archivePath, false)

	_, _, err := executeCobraText(cmd, []string{"zzzzzz"}, false)
	var cna *CardNotArchivedError
	if !errors.As(err, &cna) {
		t.Fatalf("err = %v, want *CardNotArchivedError", err)
	}
}
