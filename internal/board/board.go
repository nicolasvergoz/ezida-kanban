package board

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// CardNotFoundError is returned by MoveCard when no card has the
// requested id. The board package owns its own copy of this typed
// error so the HTTP layer (and any future board-level helper) can
// signal "no such card" without depending on internal/commands —
// which would create an import cycle (commands imports board).
//
// The CLI surface continues to use commands.CardNotFoundError for
// `ezida get` / `ezida rm`; the HTTP layer maps board's flavour to
// the same wire code CARD_NOT_FOUND in handlers.go.
type CardNotFoundError struct {
	ID string
}

func (e *CardNotFoundError) Error() string {
	return fmt.Sprintf("board: no card with id %q", e.ID)
}

// ColumnNotFoundError is returned by MoveCard when the requested
// destination column is not declared in [board].columns. See the
// rationale on CardNotFoundError for the duplication versus
// internal/commands.
type ColumnNotFoundError struct {
	Column string
}

func (e *ColumnNotFoundError) Error() string {
	return fmt.Sprintf("board: column %q is not declared in [board].columns", e.Column)
}

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
// write phase inherits a single implementation. As of V2 (drag/reorder)
// the helper delegates to InsertCardAt with position = number of cards
// already in c.Column; the observable behavior is unchanged.
func AppendCardToColumn(b *Board, c Card) {
	count := 0
	for _, existing := range b.Cards {
		if existing.Column == c.Column {
			count++
		}
	}
	InsertCardAt(b, c, c.Column, count)
}

// InsertCardAt inserts c into b.Cards so that, after the call, the
// card occupies the 0-indexed `position` among cards whose Column
// equals `column`. Sets c.Column = column before inserting. position
// is clamped to [0, N] where N is the count of cards in `column`
// after the insert (excluding any existing card with the same ID —
// relevant when called from MoveCard mid-relocation). The helper
// never returns an error: clamping makes the call total over its
// input (ADR 0002 §D11).
func InsertCardAt(b *Board, c Card, column string, position int) {
	c.Column = column

	// Build the list of flat indices currently occupied by `column`,
	// excluding any matching c.ID (caller may pass an existing card,
	// e.g. from MoveCard).
	var colIdx []int
	for i, x := range b.Cards {
		if x.Column == column && x.ID != c.ID {
			colIdx = append(colIdx, i)
		}
	}

	// Clamp position to [0, len(colIdx)].
	if position < 0 {
		position = 0
	}
	if position > len(colIdx) {
		position = len(colIdx)
	}

	// Compute insertion point in the flat slice.
	var insertAt int
	switch {
	case len(colIdx) == 0:
		// First card of an empty column → append to end of slice.
		insertAt = len(b.Cards)
	case position == len(colIdx):
		// Past the last existing same-column card → insert immediately
		// after it (preserves AppendCardToColumn semantics).
		insertAt = colIdx[len(colIdx)-1] + 1
	default:
		insertAt = colIdx[position]
	}

	b.Cards = append(b.Cards, Card{}) // grow by one
	copy(b.Cards[insertAt+1:], b.Cards[insertAt:len(b.Cards)-1])
	b.Cards[insertAt] = c
}

// MoveCard relocates the card identified by id to (column, position).
// It refreshes the moved card's UpdatedAt to the current UTC time at
// second precision before reinserting. Returns *CardNotFoundError if
// no card has the given id, or *ColumnNotFoundError if column is not
// in b.Board.Columns. Position is clamped by the underlying
// InsertCardAt (ADR 0002 §D11).
func MoveCard(b *Board, id, column string, position int) error {
	curIdx := -1
	for i, c := range b.Cards {
		if c.ID == id {
			curIdx = i
			break
		}
	}
	if curIdx < 0 {
		return &CardNotFoundError{ID: id}
	}

	// Validate the destination column against the board config.
	found := false
	for _, col := range b.Board.Columns {
		if col == column {
			found = true
			break
		}
	}
	if !found {
		return &ColumnNotFoundError{Column: column}
	}

	// Pull the card out, refresh its timestamp, and re-insert.
	c := b.Cards[curIdx]
	b.Cards = append(b.Cards[:curIdx], b.Cards[curIdx+1:]...)
	c.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	InsertCardAt(b, c, column, position)
	return nil
}
