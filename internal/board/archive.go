package board

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// CardNotArchivedError is returned when an operation targets an id that
// is not present in the archive.
type CardNotArchivedError struct {
	ID string
}

func (e *CardNotArchivedError) Error() string {
	return fmt.Sprintf("board: no archived card with id %q", e.ID)
}

// IDCollisionError is returned by UnarchiveCard when restoring would
// duplicate an id already present on the live board.
type IDCollisionError struct {
	ID string
}

func (e *IDCollisionError) Error() string {
	return fmt.Sprintf("board: id %q already exists on the board", e.ID)
}

// Archive is the in-memory representation of a kanban.archive.toml file.
type Archive struct {
	SchemaVersion int            `toml:"schema_version"`
	Cards         []ArchivedCard `toml:"cards"`
}

// ArchivedCard is a Card that has left the active board. Card is
// embedded anonymously so go-toml flattens it into a single [[cards]]
// block on both marshal and unmarshal — a field added to Card reaches
// the archive automatically, with no second struct to keep in sync
// (unlike output.ExportCard / server.cardResponse).
type ArchivedCard struct {
	Card
	ArchivedAt time.Time `toml:"archived_at"`
}

// ArchivePathFor derives the sibling archive path for a board path:
// "kanban.toml" -> "kanban.archive.toml". The archive path is always
// derived, never configured separately.
func ArchivePathFor(boardPath string) string {
	dir := filepath.Dir(boardPath)
	base := filepath.Base(boardPath)
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	return filepath.Join(dir, stem+".archive"+ext)
}

// LoadArchive reads path and parses it as TOML. A missing file is not
// an error: it yields an empty archive at SupportedSchemaVersion and
// existed=false, so every read path can treat "no archive yet" the
// same as "empty archive". A present file is validated with
// ValidateArchive before returning.
func LoadArchive(path string) (a *Archive, existed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Archive{SchemaVersion: SupportedSchemaVersion, Cards: []ArchivedCard{}}, false, nil
		}
		return nil, false, err
	}
	var loaded Archive
	if err := toml.Unmarshal(data, &loaded); err != nil {
		return nil, false, err
	}
	if verr := ValidateArchive(&loaded); verr != nil {
		return nil, false, verr
	}
	return &loaded, true, nil
}

// SaveArchive validates a and writes it atomically to path using the
// same temp-file-plus-rename technique as Save. When a has no cards,
// the file is removed instead of written — so a fully-restored archive
// is indistinguishable on disk from a board that never archived
// anything, and a missing-file remove is treated as success.
func SaveArchive(path string, a *Archive) error {
	if verr := ValidateArchive(a); verr != nil {
		return verr
	}
	if len(a.Cards) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	data, err := toml.Marshal(a)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kanban.archive.toml.tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// ExistingIDs returns every id on b plus every id in a. a may be nil,
// in which case the result equals the board's own card ids. Every
// caller that mints a new id must pass this union, so a created card
// can never collide with one waiting in the archive.
func ExistingIDs(b *Board, a *Archive) []string {
	out := make([]string, 0, len(b.Cards)+len(archiveCards(a)))
	for _, c := range b.Cards {
		out = append(out, c.ID)
	}
	for _, c := range archiveCards(a) {
		out = append(out, c.ID)
	}
	return out
}

// ReconcileArchive drops from a every card whose id also appears on b,
// returning the dropped ids in archive order. It is pure: the caller
// persists the result if it wants to heal the file. This is how a
// duplicate left behind by a crash between the two writes of an
// archive operation is resolved on read — the live board always wins.
func ReconcileArchive(b *Board, a *Archive) []string {
	if a == nil || len(a.Cards) == 0 {
		return nil
	}
	live := make(map[string]struct{}, len(b.Cards))
	for _, c := range b.Cards {
		live[c.ID] = struct{}{}
	}
	var dropped []string
	kept := a.Cards[:0:0]
	for _, c := range a.Cards {
		if _, isLive := live[c.ID]; isLive {
			dropped = append(dropped, c.ID)
			continue
		}
		kept = append(kept, c)
	}
	a.Cards = kept
	return dropped
}

// CrossFileDuplicates returns the ids present both on the live board
// and in the archive, without mutating either.
func CrossFileDuplicates(b *Board, a *Archive) []string {
	if a == nil || len(a.Cards) == 0 {
		return nil
	}
	live := make(map[string]struct{}, len(b.Cards))
	for _, c := range b.Cards {
		live[c.ID] = struct{}{}
	}
	var dup []string
	for _, c := range a.Cards {
		if _, isLive := live[c.ID]; isLive {
			dup = append(dup, c.ID)
		}
	}
	return dup
}

func archiveCards(a *Archive) []ArchivedCard {
	if a == nil {
		return nil
	}
	return a.Cards
}
