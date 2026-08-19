package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// TestList_DefaultOutputUnchangedWithArchivePresent proves the "no
// archive flags" path never reads the archive file at all: two
// otherwise-identical boards, one with a populated kanban.archive.toml
// next to it and one without, must produce byte-identical `list`
// output.
func TestList_DefaultOutputUnchangedWithArchivePresent(t *testing.T) {
	withArchive := copyFixture(t)
	without := copyFixture(t)

	at := time.Now().UTC()
	seed := &board.Archive{
		SchemaVersion: board.SupportedSchemaVersion,
		Cards: []board.ArchivedCard{{
			Card: board.Card{
				ID: "zzzzzz", Title: "junk", Column: "todo",
				CreatedAt: at, UpdatedAt: at,
			},
			ArchivedAt: at,
		}},
	}
	if err := board.SaveArchive(board.ArchivePathFor(withArchive), seed); err != nil {
		t.Fatalf("seed archive: %v", err)
	}

	textWith := captureList(t, withArchive, listFlags{}, false)
	textWithout := captureList(t, without, listFlags{}, false)
	if textWith != textWithout {
		t.Fatalf("text output differs with an archive file present:\nwith:\n%s\nwithout:\n%s", textWith, textWithout)
	}

	jsonWith := captureList(t, withArchive, listFlags{}, true)
	jsonWithout := captureList(t, without, listFlags{}, true)
	if jsonWith != jsonWithout {
		t.Fatalf("json output differs with an archive file present:\nwith:\n%s\nwithout:\n%s", jsonWith, jsonWithout)
	}
}

func TestList_IncludeArchived_OrderIsLiveThenArchived(t *testing.T) {
	path := copyFixture(t)
	archivePath := board.ArchivePathFor(path)
	archiveCmd := newDummyArchiveForPath(path, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("archive a3f2k9: %v", err)
	}
	if _, _, err := executeCobraText(archiveCmd, []string{"b7m1p4"}, false); err != nil {
		t.Fatalf("archive b7m1p4: %v", err)
	}

	stdout := captureList(t, path, listFlags{includeArchived: true}, true)
	var raw struct {
		Cards []map[string]any `json:"cards"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Cards) != 11 {
		t.Fatalf("cards = %d, want 11 (9 live + 2 archived)", len(raw.Cards))
	}
	for i := 0; i < 9; i++ {
		if _, has := raw.Cards[i]["archived_at"]; has {
			t.Fatalf("card[%d] (%v) unexpectedly archived", i, raw.Cards[i]["id"])
		}
	}
	for i := 9; i < 11; i++ {
		if _, has := raw.Cards[i]["archived_at"]; !has {
			t.Fatalf("card[%d] (%v) missing archived_at", i, raw.Cards[i]["id"])
		}
	}
	// Archive file order: most recently archived is prepended first,
	// so b7m1p4 (archived second) sits ahead of a3f2k9.
	if raw.Cards[9]["id"] != "b7m1p4" || raw.Cards[10]["id"] != "a3f2k9" {
		t.Fatalf("archived block order = [%v, %v], want [b7m1p4, a3f2k9]",
			raw.Cards[9]["id"], raw.Cards[10]["id"])
	}
}

func TestList_ArchivedOnly_AcceptsDeletedColumnFilter(t *testing.T) {
	path := copyFixture(t)
	archivePath := board.ArchivePathFor(path)
	archiveCmd := newDummyArchiveForPath(path, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("archive a3f2k9: %v", err)
	}

	// Remove "todo" (a3f2k9's stored column) from the live board so the
	// filter value exists only among archived cards.
	b, err := board.Load(path)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	b.Cards = nil
	if err := board.DeleteColumn(b, "todo"); err != nil {
		t.Fatalf("DeleteColumn: %v", err)
	}
	if err := board.Save(path, b); err != nil {
		t.Fatalf("board.Save: %v", err)
	}

	stdout := captureList(t, path, listFlags{archivedOnly: true, column: "todo"}, true)
	var raw struct {
		Cards []map[string]any `json:"cards"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Cards) != 1 || raw.Cards[0]["id"] != "a3f2k9" {
		t.Fatalf("cards = %v, want [a3f2k9]", raw.Cards)
	}
}

func TestList_BothFlags_MutuallyExclusive(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "list"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := runList(cmd, path, listFlags{includeArchived: true, archivedOnly: true}, true)
	var mex *MutuallyExclusiveFlagsError
	if !errors.As(err, &mex) {
		t.Fatalf("err = %v, want *MutuallyExclusiveFlagsError", err)
	}
}

func TestList_JSONOmitsArchivedAtForLiveCards(t *testing.T) {
	path := copyFixture(t)
	archivePath := board.ArchivePathFor(path)
	archiveCmd := newDummyArchiveForPath(path, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("archive a3f2k9: %v", err)
	}

	stdout := captureList(t, path, listFlags{includeArchived: true}, true)
	var raw struct {
		Cards []map[string]any `json:"cards"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, c := range raw.Cards {
		_, isArchived := c["archived_at"]
		if c["id"] == "a3f2k9" && !isArchived {
			t.Fatalf("archived card a3f2k9 is missing the archived_at key")
		}
		if c["id"] != "a3f2k9" && isArchived {
			t.Fatalf("live card %v unexpectedly carries the archived_at key", c["id"])
		}
	}
}

// --- archive get ---

func TestArchiveGet_LiveCardIsNotArchived(t *testing.T) {
	path := copyFixture(t)
	archivePath := board.ArchivePathFor(path)
	cmd := &cobra.Command{Use: "archive-get"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := runArchiveGet(cmd, archivePath, "a3f2k9", false)
	var cna *CardNotArchivedError
	if !errors.As(err, &cna) {
		t.Fatalf("err = %v, want *CardNotArchivedError", err)
	}
}

func TestArchiveGet_ReportsArchivedAt(t *testing.T) {
	path := copyFixture(t)
	archivePath := board.ArchivePathFor(path)
	archiveCmd := newDummyArchiveForPath(path, archivePath, false)
	if _, _, err := executeCobraText(archiveCmd, []string{"a3f2k9"}, false); err != nil {
		t.Fatalf("archive a3f2k9: %v", err)
	}

	cmd := &cobra.Command{Use: "archive-get"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runArchiveGet(cmd, archivePath, "a3f2k9", true); err != nil {
		t.Fatalf("archive get: %v", err)
	}
	var raw struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, has := raw.Card["archived_at"]; !has {
		t.Fatal("archive get JSON missing archived_at")
	}
}
