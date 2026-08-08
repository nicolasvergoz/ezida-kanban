package board

import (
	"bytes"
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
//
// v2 added Card.Epic, Card.Color and the terminal-column `*` suffix in
// [board].columns. `ezida migrate` is the only code path allowed to read
// a file at any other version.
const SupportedSchemaVersion = 2

// Board is the in-memory representation of a kanban.toml file.
type Board struct {
	SchemaVersion int         `toml:"schema_version"`
	Board         BoardConfig `toml:"board"`
	Cards         []Card      `toml:"cards"`
}

// BoardConfig holds the columns and priorities lists from [board].
//
// Columns always holds *bare* column names: the terminal-column `*`
// suffix is a serialization detail decoded by Load and re-encoded by
// Save (design D3). The decoded flags live in the unexported
// doneColumns set, reachable only through IsDoneColumn / DoneColumns /
// SetDoneColumn, so no caller can desync it from Columns. Being
// unexported also keeps it out of the TOML marshaller.
type BoardConfig struct {
	Columns        []string          `toml:"columns"`
	Priorities     []string          `toml:"priorities"`
	PriorityColors map[string]string `toml:"priority_colors,omitempty"`

	doneColumns map[string]bool
}

// DefaultPriorityColors maps the three conventional priority names to
// their default hex colors. Surfaces are filled by ResolvePriorityColors
// when the priority is declared and the user did not supply a value.
var DefaultPriorityColors = map[string]string{
	"low":    "#22c55e",
	"medium": "#f59e0b",
	"high":   "#ef4444",
}

// ResolvePriorityColors returns the final priority → color map exposed
// by wire surfaces (/api/board, `ezida export`). User values from the
// TOML always win; any conventional default name in DefaultPriorityColors
// that is declared in `priorities` but unset by the user is filled in.
// Keys not present in `priorities` are dropped (defense in depth; the
// validator already rejects them at load time). The returned map is
// always non-nil (possibly empty).
func ResolvePriorityColors(priorities []string, user map[string]string) map[string]string {
	priSet := make(map[string]struct{}, len(priorities))
	for _, p := range priorities {
		priSet[p] = struct{}{}
	}
	out := make(map[string]string, len(priorities))
	for k, v := range user {
		if _, ok := priSet[k]; ok {
			out[k] = v
		}
	}
	for name, def := range DefaultPriorityColors {
		if _, declared := priSet[name]; !declared {
			continue
		}
		if _, set := out[name]; set {
			continue
		}
		out[name] = def
	}
	return out
}

// Card is one [[cards]] entry.
//
// Epic, when non-empty, holds the id of another card on the same board
// (the "parent"). Nesting is capped at one level: a card carrying Epic
// may not itself be named as another card's Epic (validation rule 13),
// which makes reference cycles unrepresentable rather than detectable.
//
// Color, when non-empty, is a literal CSS hex string. Named palette
// entries (see colors.go) are a CLI convenience and are never written
// to disk.
type Card struct {
	ID          string    `toml:"id"`
	Title       string    `toml:"title"`
	Column      string    `toml:"column"`
	Description string    `toml:"description"`
	CreatedAt   time.Time `toml:"created_at"`
	UpdatedAt   time.Time `toml:"updated_at"`
	Tags        []string  `toml:"tags"`
	Priority    string    `toml:"priority,omitempty"`
	Epic        string    `toml:"epic,omitempty"`
	Color       string    `toml:"color,omitempty"`
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
	// Strip the terminal-column markers so every in-memory comparison
	// against Columns sees a bare name (design D3).
	names, done := DecodeColumns(b.Board.Columns)
	b.Board.Columns = names
	b.Board.doneColumns = done
	if verr := Validate(&b); verr != nil {
		return nil, verr
	}
	return &b, nil
}

// LoadUnchecked parses the file at path without enforcing the schema
// version and without validating. It exists for `ezida migrate`, which
// is by design the only command allowed to read a file the rest of the
// binary refuses (design D8) — every other caller must use Load.
//
// Terminal markers are still decoded, so a hand-written marker in an
// older file is honored rather than silently doubled.
func LoadUnchecked(path string) (*Board, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Board
	if err := toml.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	names, done := DecodeColumns(b.Board.Columns)
	b.Board.Columns = names
	b.Board.doneColumns = done
	return &b, nil
}

// Save validates b, marshals it to TOML, and writes the result atomically to
// path via a temp file in the same directory plus os.Rename.
//
// The terminal-column markers are re-attached on a copy of b, so a
// caller's board still holds bare names after the call.
func Save(path string, b *Board) error {
	if verr := Validate(b); verr != nil {
		return verr
	}
	enc := *b
	enc.Board.Columns = EncodeColumns(b.Board.Columns, b.Board.doneColumns)
	data, err := toml.Marshal(&enc)
	if err != nil {
		return err
	}
	data = withColumnsComment(data)

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

// withColumnsComment inserts ColumnsComment immediately above the
// `columns` key of the marshaled document. go-toml has no comment
// support on the encode side, so this is a line-level insertion on the
// rendered bytes. A document with no `columns` key (only possible on a
// board the validator would already reject) is returned unchanged.
func withColumnsComment(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		key := bytes.TrimLeft(line, " \t")
		if !bytes.HasPrefix(key, []byte("columns")) {
			continue
		}
		if rest := bytes.TrimLeft(key[len("columns"):], " \t"); !bytes.HasPrefix(rest, []byte("=")) {
			continue
		}
		indent := line[:len(line)-len(bytes.TrimLeft(line, " \t"))]
		comment := append(append([]byte{}, indent...), ColumnsComment...)
		out := make([][]byte, 0, len(lines)+1)
		out = append(out, lines[:i]...)
		out = append(out, comment)
		out = append(out, lines[i:]...)
		return bytes.Join(out, []byte("\n"))
	}
	return data
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
