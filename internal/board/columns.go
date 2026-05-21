package board

import (
	"fmt"
	"slices"
	"strings"
)

// affectedCard is the {id, title} pair carried by ColumnHasCardsError
// so the HTTP layer can serialize the blocking cards in the error
// envelope without re-walking the board. JSON tags match the wire
// shape consumed by the viewer (`details.cards[]`).
type affectedCard struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// EmptyColumnNameError is returned by AddColumn / RenameColumn when
// the supplied name is empty or whitespace-only after trim. The HTTP
// layer maps it to the wire code INVALID_BODY (400) per
// add-inline-column-ops design TD2.
type EmptyColumnNameError struct{}

func (e *EmptyColumnNameError) Error() string {
	return "board: column name must be non-empty"
}

// ColumnAlreadyExistsError is returned by AddColumn / RenameColumn
// when the requested name would collide with a column that already
// exists. The HTTP layer maps it to COLUMN_ALREADY_EXISTS (400) per
// ADR 0003 §D9.
type ColumnAlreadyExistsError struct {
	Name string
}

func (e *ColumnAlreadyExistsError) Error() string {
	return fmt.Sprintf("board: column %q already exists", e.Name)
}

// CannotDeleteLastColumnError is returned by DeleteColumn when the
// caller attempts to remove the only remaining column. The HTTP
// layer maps it to CANNOT_DELETE_LAST_COLUMN (400) per ADR 0003 §D9 /
// §D12 — the board MUST keep at least one column at all times.
type CannotDeleteLastColumnError struct {
	Name string
}

func (e *CannotDeleteLastColumnError) Error() string {
	return fmt.Sprintf("board: cannot delete column %q because it is the only remaining column", e.Name)
}

// ColumnHasCardsError is returned by DeleteColumn when the column
// targeted for deletion still contains one or more cards. The HTTP
// layer maps it to COLUMN_HAS_CARDS (400) per ADR 0003 §D9; the
// Cards slice carries the blocking cards as {id, title} pairs so the
// UI can surface a useful message.
type ColumnHasCardsError struct {
	Name  string
	Cards []affectedCard
}

func (e *ColumnHasCardsError) Error() string {
	return fmt.Sprintf("board: column %q still has %d card(s); move them first", e.Name, len(e.Cards))
}

// AddColumn appends a new column to b.Board.Columns. The name is
// trimmed; empty post-trim returns *EmptyColumnNameError; a
// trimmed-name collision with an existing column returns
// *ColumnAlreadyExistsError. On success the trimmed value is
// appended. AddColumn does not load or save — mutation is in-memory
// only; the caller (CLI / HTTP handler) owns persistence.
func AddColumn(b *Board, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return &EmptyColumnNameError{}
	}
	for _, existing := range b.Board.Columns {
		if existing == trimmed {
			return &ColumnAlreadyExistsError{Name: trimmed}
		}
	}
	b.Board.Columns = append(b.Board.Columns, trimmed)
	return nil
}

// RenameColumn renames a column in b.Board.Columns and cascades the
// rename across every card whose Column field equals from. The cascade
// does NOT refresh affected cards' UpdatedAt — a column rename is a
// board-level rebrand, not a card edit (design TD5).
//
// Validation order:
//
//  1. from == to → no-op success (nil).
//  2. Trimmed to is empty → *EmptyColumnNameError.
//  3. from is not in b.Board.Columns → *ColumnNotFoundError{Column: from}.
//  4. Trimmed to is already in b.Board.Columns → *ColumnAlreadyExistsError{Name: trimmed}.
//  5. Otherwise mutate b.Board.Columns[idx] = trimmed and walk b.Cards
//     rewriting every card whose Column == from to Column = trimmed.
func RenameColumn(b *Board, from, to string) error {
	if from == to {
		return nil
	}
	trimmed := strings.TrimSpace(to)
	if trimmed == "" {
		return &EmptyColumnNameError{}
	}

	idx := -1
	for i, existing := range b.Board.Columns {
		if existing == from {
			idx = i
			break
		}
	}
	if idx < 0 {
		return &ColumnNotFoundError{Column: from}
	}

	for _, existing := range b.Board.Columns {
		if existing == trimmed {
			return &ColumnAlreadyExistsError{Name: trimmed}
		}
	}

	b.Board.Columns[idx] = trimmed
	for i := range b.Cards {
		if b.Cards[i].Column == from {
			b.Cards[i].Column = trimmed
		}
	}
	return nil
}

// DeleteColumn removes a column from b.Board.Columns. DeleteColumn
// does NOT mutate b.Cards — when cards reference the column, the
// deletion is refused entirely.
//
// Validation order:
//
//  1. name is not in b.Board.Columns → *ColumnNotFoundError{Column: name}.
//  2. len(b.Board.Columns) == 1 → *CannotDeleteLastColumnError{Name: name}.
//  3. Any card has Column == name → *ColumnHasCardsError{Name: name, Cards: [...]}
//     where Cards carries every blocking card as {ID, Title}.
//  4. Otherwise delete the entry via slices.Delete.
func DeleteColumn(b *Board, name string) error {
	idx := -1
	for i, existing := range b.Board.Columns {
		if existing == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return &ColumnNotFoundError{Column: name}
	}
	if len(b.Board.Columns) == 1 {
		return &CannotDeleteLastColumnError{Name: name}
	}
	var blocking []affectedCard
	for _, c := range b.Cards {
		if c.Column == name {
			blocking = append(blocking, affectedCard{ID: c.ID, Title: c.Title})
		}
	}
	if len(blocking) > 0 {
		return &ColumnHasCardsError{Name: name, Cards: blocking}
	}
	b.Board.Columns = slices.Delete(b.Board.Columns, idx, idx+1)
	return nil
}

// MoveColumn moves the named column to a new 0-indexed position.
// position is clamped to [0, len(columns)-1] (consistent with the
// card-position clamping in InsertCardAt per ADR 0002 §D11). Cards
// are NOT touched by this operation.
//
// Validation order:
//
//  1. name is not in b.Board.Columns → *ColumnNotFoundError{Column: name}.
//  2. Clamp position to [0, len(columns)-1].
//  3. clamped target == current index → nil (no-op success).
//  4. Otherwise slice-out at current index, slice-insert at target.
func MoveColumn(b *Board, name string, position int) error {
	curIdx := -1
	for i, existing := range b.Board.Columns {
		if existing == name {
			curIdx = i
			break
		}
	}
	if curIdx < 0 {
		return &ColumnNotFoundError{Column: name}
	}
	last := len(b.Board.Columns) - 1
	target := position
	if target < 0 {
		target = 0
	}
	if target > last {
		target = last
	}
	if target == curIdx {
		return nil
	}
	// Slice-out the column at curIdx.
	cols := slices.Delete(b.Board.Columns, curIdx, curIdx+1)
	// Slice-insert at target. After deletion, target indexes into the
	// shortened slice using the same semantics: insert before the
	// element currently at `target`.
	cols = slices.Insert(cols, target, name)
	b.Board.Columns = cols
	return nil
}
