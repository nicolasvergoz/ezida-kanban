package board

import (
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// SupportedSchemaVersion is the on-disk kanban.toml schema version this
// package understands. Load refuses files at any other version with a
// SchemaVersionError; Validate enforces the same constraint.
const SupportedSchemaVersion = 1

// Board is the in-memory representation of a kanban.toml file.
type Board struct {
	SchemaVersion int         `toml:"schema_version"`
	Board         BoardConfig `toml:"board"`
	Cards         []Card      `toml:"cards"`
}

// BoardConfig holds the columns and priorities lists from [board].
type BoardConfig struct {
	Columns    []string `toml:"columns"`
	Priorities []string `toml:"priorities"`
}

// Card is one [[cards]] entry.
type Card struct {
	ID          string    `toml:"id"`
	Title       string    `toml:"title"`
	Column      string    `toml:"column"`
	Description string    `toml:"description"`
	CreatedAt   time.Time `toml:"created_at"`
	UpdatedAt   time.Time `toml:"updated_at"`
	Tags        []string  `toml:"tags"`
	Priority    string    `toml:"priority,omitempty"`
}

// Load reads the file at path, parses it as TOML, checks the schema version,
// and runs Validate before returning.
//
// Returned errors:
//   - filesystem errors from os.ReadFile (satisfying errors.Is(err, fs.ErrNotExist) when missing)
//   - the raw TOML decode error when the bytes are not valid TOML
//   - *SchemaVersionError when the file's schema_version does not match SupportedSchemaVersion
//   - *ValidationError when the file parses cleanly but breaks one or more validation rules
func Load(path string) (*Board, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Board
	if err := toml.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	if b.SchemaVersion != SupportedSchemaVersion {
		return nil, &SchemaVersionError{
			FileVersion:      b.SchemaVersion,
			SupportedVersion: SupportedSchemaVersion,
		}
	}
	if verr := Validate(&b); verr != nil {
		return nil, verr
	}
	return &b, nil
}

// Save validates b, marshals it to TOML, and writes the result atomically to
// path via a temp file in the same directory plus os.Rename.
func Save(path string, b *Board) error {
	if verr := Validate(b); verr != nil {
		return verr
	}
	data, err := toml.Marshal(b)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kanban.toml.tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if anything fails before the rename. After a
	// successful rename, tmpName no longer exists and this is a no-op.
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

// AppendCardToColumn inserts c into b.Cards immediately after the last card
// whose Column matches c.Column. If no card in c.Column exists yet, the new
// card is appended to the end of b.Cards.
//
// This codifies the "append to bottom of column" behavior (ADR §D12) so every
// write phase inherits a single implementation.
func AppendCardToColumn(b *Board, c Card) {
	lastIdx := -1
	for i, existing := range b.Cards {
		if existing.Column == c.Column {
			lastIdx = i
		}
	}
	if lastIdx == -1 {
		b.Cards = append(b.Cards, c)
		return
	}
	// Insert immediately after lastIdx.
	insertAt := lastIdx + 1
	b.Cards = append(b.Cards, Card{}) // grow by one
	copy(b.Cards[insertAt+1:], b.Cards[insertAt:len(b.Cards)-1])
	b.Cards[insertAt] = c
}
