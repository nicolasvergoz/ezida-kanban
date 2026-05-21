## ADDED Requirements

### Requirement: `AddColumn` appends a new column to the board

The package SHALL expose `AddColumn(b *Board, name string) error`
that validates `name` and appends it to `b.Board.Columns`. The
helper MUST trim `name` before validation. The helper MUST NOT call
`Load` or `Save` — mutation is in-memory only; the caller (CLI or
HTTP handler) is responsible for persistence.

Validation order:

1. If the trimmed value is empty → return `*EmptyColumnNameError`.
2. If the trimmed value already appears in `b.Board.Columns` →
   return `*ColumnAlreadyExistsError{Name: trimmed}`.
3. Otherwise, append the trimmed value to `b.Board.Columns` and
   return nil.

#### Scenario: Append a fresh column

- **WHEN** a board has columns `["todo","done"]` and
  `AddColumn(b, "review")` is called
- **THEN** `b.Board.Columns` MUST equal `["todo","done","review"]`
- **AND** the call MUST return `nil`

#### Scenario: Empty name rejected

- **WHEN** `AddColumn(b, "")` or `AddColumn(b, "   ")` is called
- **THEN** the call MUST return `*EmptyColumnNameError`
- **AND** `b.Board.Columns` MUST be unchanged

#### Scenario: Duplicate name rejected

- **WHEN** a board has columns `["todo","done"]` and
  `AddColumn(b, "todo")` is called
- **THEN** the call MUST return `*ColumnAlreadyExistsError`
- **AND** the error's `Name` field MUST equal `"todo"`
- **AND** `b.Board.Columns` MUST be unchanged

#### Scenario: Whitespace trimmed before validation

- **WHEN** a board has columns `["todo"]` and
  `AddColumn(b, "  review  ")` is called
- **THEN** `b.Board.Columns` MUST equal `["todo","review"]`

### Requirement: `RenameColumn` renames in the board and propagates to cards

The package SHALL expose
`RenameColumn(b *Board, from, to string) error` that renames a
column in `b.Board.Columns` AND rewrites every card whose `Column`
field equals `from` to reference `to` instead, in a single
in-memory mutation. The helper MUST NOT refresh affected cards'
`UpdatedAt` — column rename is a board-level rebrand, not a card
edit.

Validation order:

1. If `from == to` → return nil (no-op success).
2. If trimmed `to` is empty → return `*EmptyColumnNameError`.
3. If `from` is not in `b.Board.Columns` → return
   `*ColumnNotFoundError{Column: from}`.
4. If trimmed `to` is already in `b.Board.Columns` → return
   `*ColumnAlreadyExistsError{Name: to}`.
5. Otherwise, replace `from` with the trimmed `to` in
   `b.Board.Columns`, walk `b.Cards` and rewrite every card whose
   `Column == from` to `Column = trimmed-to`, and return nil.

#### Scenario: Rename propagates to every referencing card

- **WHEN** a board has columns `["todo","done"]` and 3 cards with
  `Column="todo"` and `RenameColumn(b, "todo", "backlog")` is
  called
- **THEN** `b.Board.Columns` MUST equal `["backlog","done"]`
- **AND** all 3 previously-`todo` cards' `Column` field MUST equal
  `"backlog"`
- **AND** each affected card's `UpdatedAt` MUST be unchanged

#### Scenario: Rename to identical name is a no-op success

- **WHEN** `RenameColumn(b, "todo", "todo")` is called
- **THEN** the call MUST return `nil`
- **AND** `b.Board.Columns` MUST be unchanged
- **AND** `b.Cards` MUST be unchanged

#### Scenario: Unknown source column rejected

- **WHEN** `RenameColumn(b, "ghost", "backlog")` is called and
  `ghost` is not in `b.Board.Columns`
- **THEN** the call MUST return `*ColumnNotFoundError`
- **AND** `b.Board.Columns` MUST be unchanged
- **AND** `b.Cards` MUST be unchanged

#### Scenario: New name already exists rejected

- **WHEN** `RenameColumn(b, "todo", "done")` is called and both
  exist
- **THEN** the call MUST return `*ColumnAlreadyExistsError`
- **AND** the error's `Name` MUST equal `"done"`
- **AND** `b.Board.Columns` MUST be unchanged
- **AND** `b.Cards` MUST be unchanged

#### Scenario: Empty new name rejected

- **WHEN** `RenameColumn(b, "todo", "")` or
  `RenameColumn(b, "todo", "   ")` is called
- **THEN** the call MUST return `*EmptyColumnNameError`
- **AND** `b.Board.Columns` MUST be unchanged

#### Scenario: Column order preserved across rename

- **WHEN** a board has columns `["a","b","c"]` and
  `RenameColumn(b, "b", "B2")` is called
- **THEN** `b.Board.Columns` MUST equal `["a","B2","c"]`

### Requirement: `DeleteColumn` removes a column when safe

The package SHALL expose `DeleteColumn(b *Board, name string) error`
that removes `name` from `b.Board.Columns` only when no card
references it and at least one other column remains. The helper
MUST NOT mutate `b.Cards`.

Validation order:

1. If `name` is not in `b.Board.Columns` → return
   `*ColumnNotFoundError{Column: name}`.
2. If `len(b.Board.Columns) == 1` → return
   `*CannotDeleteLastColumnError{Name: name}`.
3. Collect every card whose `Column == name`. If any exist →
   return `*ColumnHasCardsError{Name: name, Cards: []affectedCard{...}}`.
4. Otherwise, delete the entry from `b.Board.Columns` and return
   nil.

The package SHALL declare an internal `affectedCard` struct with at
least `ID string` and `Title string` fields (JSON-tagged as `id`
and `title` for the HTTP layer's pass-through).

#### Scenario: Delete an empty column

- **WHEN** a board has columns `["todo","done","review"]` and no
  card references `review`, and `DeleteColumn(b, "review")` is
  called
- **THEN** `b.Board.Columns` MUST equal `["todo","done"]`
- **AND** `b.Cards` MUST be unchanged
- **AND** the call MUST return `nil`

#### Scenario: Unknown column rejected

- **WHEN** `DeleteColumn(b, "ghost")` is called and `ghost` is not
  in `b.Board.Columns`
- **THEN** the call MUST return `*ColumnNotFoundError`
- **AND** the error's `Column` MUST equal `"ghost"`
- **AND** `b.Board.Columns` MUST be unchanged

#### Scenario: Last column refused

- **WHEN** a board has columns `["todo"]` and no card references
  `todo`, and `DeleteColumn(b, "todo")` is called
- **THEN** the call MUST return `*CannotDeleteLastColumnError`
- **AND** the error's `Name` MUST equal `"todo"`
- **AND** `b.Board.Columns` MUST be unchanged

#### Scenario: Column with cards refused

- **WHEN** a board has columns `["todo","done"]` and 2 cards with
  `Column="todo"`, and `DeleteColumn(b, "todo")` is called
- **THEN** the call MUST return `*ColumnHasCardsError`
- **AND** the error's `Name` MUST equal `"todo"`
- **AND** the error's `Cards` slice MUST contain exactly 2 entries
- **AND** each entry MUST carry the corresponding card's `ID` and
  `Title`
- **AND** `b.Board.Columns` MUST be unchanged
- **AND** `b.Cards` MUST be unchanged

#### Scenario: Column order preserved across delete

- **WHEN** a board has columns `["a","b","c"]` and no card
  references `b`, and `DeleteColumn(b, "b")` is called
- **THEN** `b.Board.Columns` MUST equal `["a","c"]`

### Requirement: `MoveColumn` reorders a column to a new index

The package SHALL expose
`MoveColumn(b *Board, name string, position int) error` that moves
the named column to a new 0-indexed position in `b.Board.Columns`.
`position` MUST be clamped to `[0, len(columns)-1]` (consistent with
the card-position clamping in `InsertCardAt` per ADR 0002 §D11).
The helper MUST NOT mutate `b.Cards`.

Validation order:

1. If `name` is not in `b.Board.Columns` → return
   `*ColumnNotFoundError{Column: name}`.
2. Clamp `position` to `[0, len(b.Board.Columns)-1]`.
3. If the clamped target equals the column's current index → return
   nil (no-op success).
4. Otherwise, remove the column from its current index and insert
   it at the target index.

#### Scenario: Move to first position

- **WHEN** a board has columns `["todo","ongoing","done"]` and
  `MoveColumn(b, "done", 0)` is called
- **THEN** `b.Board.Columns` MUST equal `["done","todo","ongoing"]`

#### Scenario: Move to middle position

- **WHEN** a board has columns `["a","b","c","d"]` and
  `MoveColumn(b, "d", 1)` is called
- **THEN** `b.Board.Columns` MUST equal `["a","d","b","c"]`

#### Scenario: Move to last position

- **WHEN** a board has columns `["a","b","c"]` and
  `MoveColumn(b, "a", 2)` is called
- **THEN** `b.Board.Columns` MUST equal `["b","c","a"]`

#### Scenario: Position past the end clamps to last index

- **WHEN** a board has columns `["a","b","c"]` and
  `MoveColumn(b, "a", 999)` is called
- **THEN** `b.Board.Columns` MUST equal `["b","c","a"]`
- **AND** the call MUST return `nil`

#### Scenario: Negative position clamps to 0

- **WHEN** a board has columns `["a","b","c"]` and
  `MoveColumn(b, "c", -5)` is called
- **THEN** `b.Board.Columns` MUST equal `["c","a","b"]`
- **AND** the call MUST return `nil`

#### Scenario: Move to same position is a no-op

- **WHEN** a board has columns `["a","b","c"]` and
  `MoveColumn(b, "b", 1)` is called
- **THEN** `b.Board.Columns` MUST equal `["a","b","c"]`
- **AND** the call MUST return `nil`

#### Scenario: Unknown column rejected

- **WHEN** `MoveColumn(b, "ghost", 0)` is called and `ghost` is not
  in `b.Board.Columns`
- **THEN** the call MUST return `*ColumnNotFoundError`
- **AND** `b.Board.Columns` MUST be unchanged

#### Scenario: Cards untouched by move

- **WHEN** any successful `MoveColumn` call completes
- **THEN** `b.Cards` MUST be byte-deep-equal to its pre-call value

### Requirement: Column-helper error types are structured

The package SHALL declare the following typed errors for the column
helpers, in addition to the existing `*ColumnNotFoundError`:

- `EmptyColumnNameError` — returned when a column name is empty or
  whitespace-only after trim. The HTTP layer maps it to wire code
  `INVALID_BODY` (400) per `add-inline-column-ops` design TD2.
- `ColumnAlreadyExistsError{Name string}` — returned when a column
  name would collide with an existing entry. The HTTP layer maps it
  to wire code `COLUMN_ALREADY_EXISTS` (400) per ADR 0003 §D9.
- `CannotDeleteLastColumnError{Name string}` — returned when
  deletion would leave `b.Board.Columns` empty. The HTTP layer maps
  it to wire code `CANNOT_DELETE_LAST_COLUMN` (400) per ADR 0003
  §D9 / §D12.
- `ColumnHasCardsError{Name string, Cards []affectedCard}` —
  returned when deletion is refused because cards still reference
  the column. The HTTP layer maps it to wire code
  `COLUMN_HAS_CARDS` (400) per ADR 0003 §D9. The `Cards` slice
  carries `{ID, Title}` entries for the blocking cards.

Each typed error MUST satisfy the `error` interface via an
`Error() string` method, and MUST carry enough context for the
HTTP layer's `httpError` to populate the `error.details` payload
without re-walking the board.

#### Scenario: EmptyColumnNameError satisfies error interface

- **WHEN** `(&EmptyColumnNameError{}).Error()` is called
- **THEN** the returned string MUST be non-empty
- **AND** the message MUST describe an empty column name

#### Scenario: ColumnAlreadyExistsError carries the name

- **WHEN** `(&ColumnAlreadyExistsError{Name: "todo"}).Error()` is
  called
- **THEN** the returned string MUST contain the literal `"todo"`

#### Scenario: CannotDeleteLastColumnError carries the name

- **WHEN** `(&CannotDeleteLastColumnError{Name: "todo"}).Error()`
  is called
- **THEN** the returned string MUST contain the literal `"todo"`

#### Scenario: ColumnHasCardsError carries blocking cards

- **WHEN** a `ColumnHasCardsError` is constructed with 3 blocking
  cards
- **THEN** the error's `Cards` slice MUST contain exactly 3 entries
- **AND** each entry MUST have a non-empty `ID`
- **AND** the `Error()` message SHOULD mention the count of
  blocking cards
