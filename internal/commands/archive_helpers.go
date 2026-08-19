package commands

import (
	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// loadArchive reads the archive at path. A missing file is not an
// error: it yields an empty archive, exactly like board.LoadArchive.
func loadArchive(path string) (*board.Archive, error) {
	a, _, err := board.LoadArchive(path)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// mutateArchiveAndSave loads both files, applies mutate, and persists
// the result with the archive written first and the board second — the
// destination of an archive operation gains the cards before the
// source loses them, so a crash between the two writes can only
// duplicate a card, never lose one (design "Write the destination
// before the source"). If mutate returns an error, neither file is
// written.
func mutateArchiveAndSave(boardPath, archivePath string, mutate func(b *board.Board, a *board.Archive) error) error {
	b, err := board.Load(boardPath)
	if err != nil {
		return err
	}
	a, err := loadArchive(archivePath)
	if err != nil {
		return err
	}
	if err := mutate(b, a); err != nil {
		return err
	}
	if err := board.SaveArchive(archivePath, a); err != nil {
		return err
	}
	return board.Save(boardPath, b)
}

// mutateUnarchiveAndSave is mutateArchiveAndSave's inverse: the board
// is written first and the archive second, because restoring gains
// cards on the board before losing them from the archive.
func mutateUnarchiveAndSave(boardPath, archivePath string, mutate func(b *board.Board, a *board.Archive) error) error {
	b, err := board.Load(boardPath)
	if err != nil {
		return err
	}
	a, err := loadArchive(archivePath)
	if err != nil {
		return err
	}
	if err := mutate(b, a); err != nil {
		return err
	}
	if err := board.Save(boardPath, b); err != nil {
		return err
	}
	return board.SaveArchive(archivePath, a)
}
