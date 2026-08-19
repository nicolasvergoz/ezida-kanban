package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// TestRoundTrip_AddMoveRm initializes a fresh board, adds a card,
// moves it across two columns, removes it, and verifies the final
// file is byte-identical to the post-init state.
func TestRoundTrip_AddMoveRm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")

	// 1. init.
	initCmd := newDummyInitForPath(path, false)
	if _, _, err := executeCobraText(initCmd, []string{}, false); err != nil {
		t.Fatalf("init: %v", err)
	}
	postInit, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read post-init: %v", err)
	}

	// 2. add a card.
	addCmd := newDummyAddForPath(path, false)
	stdout, _, err := executeCobraText(addCmd, []string{"T1", "--column=todo"}, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	id := strings.TrimSpace(stdout)
	if id == "" {
		t.Fatalf("empty id")
	}
	// Verify card exists.
	b, _ := board.Load(path)
	if len(b.Cards) != 1 || b.Cards[0].ID != id || b.Cards[0].Column != "todo" {
		t.Errorf("post-add: %+v", b.Cards)
	}

	// 3. move to ongoing.
	moveCmd := newDummyMoveForPath(path, false)
	if _, _, err := executeCobraText(moveCmd, []string{id, "ongoing"}, false); err != nil {
		t.Fatalf("move: %v", err)
	}
	b, _ = board.Load(path)
	if b.Cards[0].Column != "ongoing" {
		t.Errorf("post-move column: %s", b.Cards[0].Column)
	}

	// 4. move to done.
	moveCmd2 := newDummyMoveForPath(path, false)
	if _, _, err := executeCobraText(moveCmd2, []string{id, "done"}, false); err != nil {
		t.Fatalf("move 2: %v", err)
	}
	b, _ = board.Load(path)
	if b.Cards[0].Column != "done" {
		t.Errorf("post-move2 column: %s", b.Cards[0].Column)
	}

	// 5. rm with --yes.
	rio := rmIO{in: nil, err: &bytes.Buffer{}, interactive: false}
	rmCmd, _ := newDummyRmForPath(path, false, rio)
	if _, _, err := executeCobraText(rmCmd, []string{id, "--yes"}, false); err != nil {
		t.Fatalf("rm: %v", err)
	}

	// 6. final file should equal post-init.
	final, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	if !bytes.Equal(postInit, final) {
		t.Errorf("final file differs from post-init:\n--- post-init ---\n%s\n--- final ---\n%s",
			postInit, final)
	}
}

// TestRoundTrip_ArchiveUnarchive initializes a fresh board, adds a
// card, archives it, then unarchives it, and verifies kanban.toml is
// byte-identical to its post-add state and that no archive file
// remains — the archive/unarchive analogue of TestRoundTrip_AddMoveRm.
func TestRoundTrip_ArchiveUnarchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	archivePath := board.ArchivePathFor(path)

	initCmd := newDummyInitForPath(path, false)
	if _, _, err := executeCobraText(initCmd, []string{}, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	addCmd := newDummyAddForPath(path, false)
	stdout, _, err := executeCobraText(addCmd, []string{"T1", "--column=todo"}, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	id := strings.TrimSpace(stdout)
	postAdd, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read post-add: %v", err)
	}

	archiveCmd := newDummyArchiveForPath(path, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{id}, false); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("archive file missing after archive: %v", err)
	}

	unarchiveCmd, _ := newDummyUnarchiveForPath(path, archivePath, false)
	if _, _, err := executeCobraText(unarchiveCmd, []string{id}, false); err != nil {
		t.Fatalf("unarchive: %v", err)
	}

	final, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	if !bytes.Equal(postAdd, final) {
		t.Errorf("final file differs from post-add:\n--- post-add ---\n%s\n--- final ---\n%s",
			postAdd, final)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatal("archive file still present after a full round trip")
	}
}

// silenceCobra is a small helper if direct cobra.Command invocation is
// needed elsewhere.
var _ = (*cobra.Command)(nil)
