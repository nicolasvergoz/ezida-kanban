package commands

import (
	"testing"
	"time"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

func TestMutateArchiveAndSave_WritesArchiveBeforeBoard(t *testing.T) {
	path := copyFixture(t)
	archivePath := board.ArchivePathFor(path)

	err := mutateArchiveAndSave(path, archivePath, func(b *board.Board, a *board.Archive) error {
		idx := -1
		for i, c := range b.Cards {
			if c.ID == "a3f2k9" {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatal("fixture card a3f2k9 not found")
		}
		card := b.Cards[idx]
		b.Cards = append(b.Cards[:idx], b.Cards[idx+1:]...)
		a.Cards = append(a.Cards, board.ArchivedCard{Card: card, ArchivedAt: time.Now().UTC()})

		// Simulate a failure in the SECOND write (the board) by leaving
		// b invalid — board.Save runs board.Validate first and refuses,
		// so the board file is never touched.
		b.SchemaVersion = 999
		return nil
	})
	if err == nil {
		t.Fatal("mutateArchiveAndSave succeeded despite an invalid board, want an error")
	}

	// The archive write (first) must have gone through.
	a, err := loadArchive(archivePath)
	if err != nil {
		t.Fatalf("loadArchive: %v", err)
	}
	found := false
	for _, c := range a.Cards {
		if c.ID == "a3f2k9" {
			found = true
		}
	}
	if !found {
		t.Fatal("archive write did not happen before the failed board write")
	}

	// The board write (second) failed, so the card is still present in
	// kanban.toml too — a duplicate, not a loss.
	liveBoard, err := board.Load(path)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	liveFound := false
	for _, c := range liveBoard.Cards {
		if c.ID == "a3f2k9" {
			liveFound = true
		}
	}
	if !liveFound {
		t.Fatal("card missing from the live board after a failed board write — this is a LOSS, not the documented duplicate")
	}
}

func TestMutateUnarchiveAndSave_WritesBoardBeforeArchive(t *testing.T) {
	path := copyFixture(t)
	archivePath := board.ArchivePathFor(path)

	at := time.Now().UTC()
	seed := &board.Archive{
		SchemaVersion: board.SupportedSchemaVersion,
		Cards: []board.ArchivedCard{{
			Card: board.Card{
				ID: "dddddd", Title: "restored", Column: "todo",
				CreatedAt: at, UpdatedAt: at,
			},
			ArchivedAt: at,
		}},
	}
	if err := board.SaveArchive(archivePath, seed); err != nil {
		t.Fatalf("seed archive: %v", err)
	}

	err := mutateUnarchiveAndSave(path, archivePath, func(b *board.Board, a *board.Archive) error {
		idx := -1
		for i, c := range a.Cards {
			if c.ID == "dddddd" {
				idx = i
				break
			}
		}
		if idx < 0 {
			t.Fatal("seeded archive card not found")
		}
		card := a.Cards[idx].Card
		a.Cards = append(a.Cards[:idx], a.Cards[idx+1:]...)
		b.Cards = append(b.Cards, card)

		// Simulate a failure in the SECOND write (the archive) by
		// leaving it invalid — board.SaveArchive runs ValidateArchive
		// first and refuses, so the archive file is never touched.
		a.SchemaVersion = 999
		return nil
	})
	if err == nil {
		t.Fatal("mutateUnarchiveAndSave succeeded despite an invalid archive, want an error")
	}

	// The board write (first) must have gone through.
	liveBoard, err := board.Load(path)
	if err != nil {
		t.Fatalf("board.Load: %v", err)
	}
	found := false
	for _, c := range liveBoard.Cards {
		if c.ID == "dddddd" {
			found = true
		}
	}
	if !found {
		t.Fatal("board write did not happen before the failed archive write")
	}

	// The archive write (second) failed, so the card is still present
	// in kanban.archive.toml too — a duplicate, not a loss.
	archived, err := loadArchive(archivePath)
	if err != nil {
		t.Fatalf("loadArchive: %v", err)
	}
	archivedFound := false
	for _, c := range archived.Cards {
		if c.ID == "dddddd" {
			archivedFound = true
		}
	}
	if !archivedFound {
		t.Fatal("card missing from the archive after a failed archive write — this is a LOSS, not the documented duplicate")
	}
}
