# Board Storage Specification

## Purpose

Persist a Kanban board to disk as a single TOML file with atomic writes, schema-version checks, ID generation, and a nine-rule validator. This is the foundation every other phase builds on.

## Requirements

### Requirement: File schema and on-disk format

The system SHALL persist a Kanban board as a single UTF-8 encoded
`kanban.toml` file using TOML v1.0. The schema MUST follow
`refs/PROJECT_BRIEF.md` §5: a top-level `schema_version` integer, a `[board]`
table with `columns` and `priorities` string arrays, an OPTIONAL
`[board.priority_colors]` inline table mapping priority name → hex color
string, and zero or more `[[cards]]` array-of-table entries with the
fields `id`, `title`, `column`, `description`, `created_at`,
`updated_at`, `tags`, and optional `priority`.

#### Scenario: Round-trip preserves all fields

- **WHEN** a valid `kanban.toml` fixture is loaded and then saved without
  modification
- **THEN** the resulting file MUST contain the same `schema_version`,
  the same `[board]` arrays in the same order, the same
  `[board.priority_colors]` entries when present, the same `[[cards]]`
  blocks in the same order, and the same field values for every card

#### Scenario: Card order is preserved across writes

- **WHEN** a board with three cards `[a, b, c]` in the same column is
  loaded, an unrelated field on card `b` is mutated, and the board is
  saved
- **THEN** the saved file's `[[cards]]` blocks MUST appear in the order
  `[a, b, c]`

#### Scenario: Column order is preserved across writes

- **WHEN** a board with columns `["todo", "ongoing", "done"]` is loaded
  and saved
- **THEN** the saved `[board].columns` array MUST be exactly
  `["todo", "ongoing", "done"]`

### Requirement: Schema version compatibility check

`Load` SHALL refuse to return a `*Board` when the file's `schema_version`
does not equal the version supported by the binary. It MUST return an error
of type `SchemaVersionError` carrying both the file's version and the
supported one.

#### Scenario: Mismatched schema version

- **WHEN** a file with `schema_version = 2` is loaded by a binary that
  supports version 1
- **THEN** `Load` MUST return a non-nil error of type `SchemaVersionError`
- **AND** the error MUST report file version `2` and supported version `1`
- **AND** no `*Board` value is returned

#### Scenario: Matching schema version

- **WHEN** a file with `schema_version = 1` is loaded by a binary that
  supports version 1
- **THEN** `Load` MUST return a populated `*Board` and a nil error
  (assuming the rest of the file passes validation)

### Requirement: Atomic persistence

`Save` SHALL write the board to disk atomically. It MUST write the new
contents to a temporary file in the same directory and rename the temp file
over the target. The target file MUST NEVER exist in a half-written state.

#### Scenario: Successful save replaces the file

- **WHEN** `Save("kanban.toml", b)` is called and succeeds
- **THEN** `kanban.toml` MUST contain the marshaled board
- **AND** no `.kanban.toml.tmp` file MUST remain in the directory

#### Scenario: Failed marshal leaves original file untouched

- **WHEN** marshaling fails before the rename step
- **THEN** the original `kanban.toml` MUST be unchanged
- **AND** the temp file MUST be removed if it was created

### Requirement: ID format and generation

Card IDs SHALL be exactly six characters drawn uniformly from the alphabet
`[0-9a-z]`. The package MUST expose `NewID() string` for unconditional
generation and `NewUniqueID(existing []string) (string, error)` that retries
up to ten times against the provided set and returns `ErrIDExhausted` on
exhaustion.

#### Scenario: NewID format

- **WHEN** `NewID()` is called
- **THEN** the returned string MUST match the regular expression
  `^[0-9a-z]{6}$`

#### Scenario: NewUniqueID avoids collisions

- **WHEN** `NewUniqueID(existing)` is called with a non-empty `existing`
  slice
- **THEN** the returned ID MUST NOT appear in `existing`

#### Scenario: NewUniqueID gives up after ten attempts

- **WHEN** `NewUniqueID` is invoked against a synthetic `existing` set
  that covers all 36⁶ values
- **THEN** the function MUST return `ErrIDExhausted`

### Requirement: Validation enforces the nine business rules

`Validate(b *Board)` SHALL return a non-nil `*ValidationError` when any of
the ten rules below is violated, and `nil` otherwise. The error MUST
enumerate all violations found in a single pass (no early return on the
first failure).

The ten rules:
1. `schema_version` equals the supported version.
2. `[board].columns` is non-empty and contains no duplicates.
3. `[board].priorities` is non-empty and contains no duplicates.
4. Every card's `id` matches `^[0-9a-z]{6}$`.
5. Card IDs are unique across the board.
6. Every card's `title` is non-empty.
7. Every card's `column` exists in `[board].columns`.
8. Every card's `priority`, when present, exists in `[board].priorities`.
9. `created_at` and `updated_at` are non-zero timestamps and
   `updated_at >= created_at`.
10. Every key of `[board].priority_colors`, when the map is non-empty,
    exists in `[board].priorities`; every value matches
    `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`.

#### Scenario: Valid board passes

- **WHEN** `Validate` is called on a board that satisfies all ten rules
- **THEN** it MUST return `nil`

#### Scenario: Duplicate card IDs are reported

- **WHEN** `Validate` is called on a board whose cards include two entries
  with `id = "a3f2k9"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 5 and reference both offending cards

#### Scenario: Card references unknown column

- **WHEN** `Validate` is called on a board whose card has
  `column = "wip"` but `[board].columns = ["todo", "done"]`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 7 and name the offending card and
  the missing column

#### Scenario: Card references unknown priority

- **WHEN** `Validate` is called on a board whose card has
  `priority = "urgent"` but `[board].priorities = ["low", "high"]`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 8

#### Scenario: Multiple violations in one pass

- **WHEN** `Validate` is called on a board that violates rules 6 and 7
- **THEN** it MUST return one `*ValidationError` whose details list both
  violations

#### Scenario: priority_colors key not in declared priorities

- **WHEN** `Validate` is called on a board whose
  `[board].priorities = ["low", "high"]` but
  `[board.priority_colors]` contains the key `"urgent"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 10 and name the offending key

#### Scenario: priority_colors value is not a hex color

- **WHEN** `Validate` is called on a board whose
  `[board.priority_colors]` contains `low = "red"`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST mention rule 10 and name the offending value

### Requirement: Optional `[board.priority_colors]` maps priorities to hex colors

`BoardConfig` SHALL accept an optional TOML inline table
`[board.priority_colors]` mapping priority name → hex color string.
The mapping MUST be modeled as `map[string]string` and serialized
under the TOML key `priority_colors` with `omitempty` semantics —
an absent or empty map MUST NOT be written back to disk as an empty
table on `Save`.

Each entry has two constraints, enforced by `Validate`:

- The key MUST equal a value declared in `[board].priorities`.
- The value MUST match the regular expression
  `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$` (three- or six-digit CSS hex,
  with leading `#`).

The mapping MAY be absent, empty, or a strict subset of declared
priorities. Priorities not present in the map have no configured
color (the viewer applies its default badge skin).

#### Scenario: Round-trip preserves a non-empty mapping

- **WHEN** a `kanban.toml` with
  `[board.priority_colors]\nlow = "#22c55e"\nhigh = "#ef4444"` is
  loaded and then saved without modification
- **THEN** the saved file MUST contain the same two entries under
  `[board.priority_colors]`

#### Scenario: Absent mapping round-trips as absent

- **WHEN** a `kanban.toml` without `[board.priority_colors]` is
  loaded and then saved without modification
- **THEN** the saved file MUST NOT contain a `[board.priority_colors]`
  table

#### Scenario: Empty mapping is legal

- **WHEN** a board loads with `priority_colors` unset
- **THEN** `Validate` MUST return `nil` for that field

### Requirement: `InsertCardAt` places a card at an explicit position within a column

The package SHALL expose `InsertCardAt(b *Board, c Card, column string, position int)` that mutates `b.Cards` so that, after the call, `c` (with `c.Column` set to `column`) occupies the 0-indexed `position` among cards whose `Column == column`. `position` MUST be clamped to `[0, N]` where `N` is the count of cards in `column` after the insert (excluding any existing card with the same `c.ID`). The helper MUST NOT return an error; clamping makes the call total over its input.

#### Scenario: Insert into the middle of a column

- **WHEN** a board has cards `[A(todo), B(todo), C(todo)]` and `InsertCardAt(b, D, "todo", 1)` is called
- **THEN** `b.Cards` MUST equal `[A(todo), D(todo), B(todo), C(todo)]`

#### Scenario: Insert at position 0 of a non-empty column

- **WHEN** a board has cards `[A(todo)]` and `InsertCardAt(b, B, "todo", 0)` is called
- **THEN** `b.Cards` MUST equal `[B(todo), A(todo)]`

#### Scenario: Insert at position past the end clamps to end

- **WHEN** a board has cards `[A(todo), B(todo)]` and `InsertCardAt(b, C, "todo", 99)` is called
- **THEN** `b.Cards` MUST equal `[A(todo), B(todo), C(todo)]`

#### Scenario: Insert at negative position clamps to zero

- **WHEN** a board has cards `[A(todo)]` and `InsertCardAt(b, B, "todo", -5)` is called
- **THEN** `b.Cards` MUST equal `[B(todo), A(todo)]`

#### Scenario: Insert into an empty column appends to end of slice

- **WHEN** a board has cards `[A(todo)]` and `InsertCardAt(b, B, "done", 0)` is called
- **THEN** `b.Cards` MUST equal `[A(todo), B(done)]`

#### Scenario: Insert sets the card's column

- **WHEN** `InsertCardAt(b, Card{ID: "x", Column: "todo"}, "done", 0)` is called
- **THEN** the inserted card's `Column` MUST equal `"done"`

### Requirement: `MoveCard` relocates a card to a new column and position

The package SHALL expose `MoveCard(b *Board, id, column string, position int) error` that removes the card identified by `id` from its current position in `b.Cards`, sets its `Column` to `column`, refreshes `UpdatedAt` to the current UTC time at second precision, and re-inserts it at the given `position` within `column` (applying the same clamping rules as `InsertCardAt`). Returns `*CardNotFoundError` if no card has `id`; returns `*ColumnNotFoundError` if `column` is not present in `b.Board.Columns`.

#### Scenario: Move across columns to a specific position

- **WHEN** a board has cards `[A(todo), B(done), C(todo)]`, columns include `done`, and `MoveCard(b, "C-id", "done", 0)` is called
- **THEN** `b.Cards` MUST equal `[A(todo), C(done), B(done)]`
- **AND** the moved card's `UpdatedAt` MUST be strictly later than its `CreatedAt`

#### Scenario: Reorder within the same column

- **WHEN** a board has cards `[A(todo), B(todo), C(todo)]` and `MoveCard(b, "A-id", "todo", 2)` is called
- **THEN** `b.Cards` MUST equal `[B(todo), C(todo), A(todo)]`

#### Scenario: Move with unknown card id

- **WHEN** `MoveCard(b, "zzzzzz", "todo", 0)` is called and no card has id `zzzzzz`
- **THEN** the call MUST return `*CardNotFoundError`
- **AND** `b.Cards` MUST be unchanged

#### Scenario: Move to an unknown column

- **WHEN** `MoveCard(b, "<existing-id>", "ghost", 0)` is called and `ghost` is not in `b.Board.Columns`
- **THEN** the call MUST return `*ColumnNotFoundError`
- **AND** `b.Cards` MUST be unchanged

#### Scenario: Move to same column same position is a no-op write

- **WHEN** a card occupies position 1 of `todo` and `MoveCard(b, id, "todo", 1)` is called
- **THEN** the card's slice index in `b.Cards` MUST be unchanged
- **AND** the card's `UpdatedAt` MUST be refreshed to the current UTC time at second precision

### Requirement: Default placement appends to the end of a column

The package SHALL expose a helper `AppendCardToColumn(b *Board, c Card)` that places the card at the position immediately after the last card whose `column` matches `c.Column`, or at the end of `b.Cards` if no such card exists. This codifies the "append to bottom" behavior (ADR §D12) so write phases inherit a single implementation. Implementation MAY delegate to `InsertCardAt`; observable behavior MUST be identical to the pre-V2 version.

#### Scenario: Card appended after existing same-column cards

- **WHEN** a board has cards `[A(todo), B(done), C(todo)]` (in this slice order) and a new card `D(todo)` is appended via `AppendCardToColumn`
- **THEN** `b.Cards` MUST equal `[A(todo), B(done), C(todo), D(todo)]`

#### Scenario: First card in a column appends to end of slice

- **WHEN** a board has cards `[A(todo)]` and a new card `B(done)` is appended via `AppendCardToColumn`
- **THEN** `b.Cards` MUST equal `[A(todo), B(done)]`

#### Scenario: AppendCardToColumn matches InsertCardAt-at-end

- **WHEN** the same fresh card is appended via `AppendCardToColumn(b, c)` and the equivalent `InsertCardAt(b, c, c.Column, count)` call (where `count` is the number of cards in `c.Column` before insert)
- **THEN** the resulting `b.Cards` slices MUST be deep-equal

### Requirement: Structured error types for downstream consumers

Failures returned by this package MUST be one of: `SchemaVersionError`,
`*ValidationError`, `ErrIDExhausted`, or an `error` wrapping a stdlib I/O
error from the underlying filesystem. Each typed error MUST carry enough
context (file version, list of violations, etc.) for a CLI layer to map
the failure to a stable error code per ADR §D8.

#### Scenario: ValidationError lists all violations

- **WHEN** validation fails with three rule violations
- **THEN** the returned `*ValidationError` MUST expose a slice of three
  violation entries, each naming the rule number and the offending element

#### Scenario: I/O failure is surfaced without wrapping into a typed error

- **WHEN** `Load` is called against a path that does not exist
- **THEN** the returned error MUST satisfy `errors.Is(err, fs.ErrNotExist)`

### Requirement: `UpdateCard` applies a partial patch to a card

The package SHALL expose `UpdateCard(b *Board, id string, p CardPatch) error` that mutates the card identified by `id` according to `p`. Each non-nil field in `p` MUST replace the corresponding card field; nil fields MUST leave the card field unchanged. The helper MUST refresh `UpdatedAt` to the current UTC time at second precision and MUST call `Validate(b)` after the mutation, returning the validation error if any rule fails.

Pre-mutation rule checks (in order):

- If `p.Title != nil` and the trimmed value is empty, return `*MissingTitleError`.
- If `p.Tags != nil` and any element's trimmed value is empty, return `*InvalidTagError`.
- If `p.Priority != nil` and the value is non-empty but not present in `b.Board.Priorities`, return `*InvalidPriorityError`.

If `id` does not match any card, return `*CardNotFoundError` before any mutation.

#### Scenario: Patch only the title

- **WHEN** a card has `Title="Old"`, `Description="x"`, `Tags=["a"]`, `Priority="low"` and `UpdateCard(b, id, CardPatch{Title: ptr("New")})` is called
- **THEN** the card's `Title` MUST equal `"New"`
- **AND** the card's `Description`, `Tags`, and `Priority` MUST be unchanged
- **AND** the card's `UpdatedAt` MUST be refreshed

#### Scenario: Patch clears a field via empty value

- **WHEN** a card has `Priority="high"` and `UpdateCard(b, id, CardPatch{Priority: ptr("")})` is called
- **THEN** the card's `Priority` MUST equal `""`

#### Scenario: Patch clears tags via empty slice

- **WHEN** a card has `Tags=["a","b"]` and `UpdateCard(b, id, CardPatch{Tags: ptrSlice([]string{})})` is called
- **THEN** the card's `Tags` MUST equal `[]`

#### Scenario: Patch with empty title is rejected

- **WHEN** `UpdateCard(b, id, CardPatch{Title: ptr("   ")})` is called
- **THEN** the call MUST return `*MissingTitleError`
- **AND** the card MUST be unchanged

#### Scenario: Patch with unknown priority is rejected

- **WHEN** `UpdateCard(b, id, CardPatch{Priority: ptr("urgent")})` is called and `urgent` is not in `b.Board.Priorities`
- **THEN** the call MUST return `*InvalidPriorityError`
- **AND** the card MUST be unchanged

#### Scenario: Patch with empty-string tag is rejected

- **WHEN** `UpdateCard(b, id, CardPatch{Tags: ptrSlice([]string{"a", ""})})` is called
- **THEN** the call MUST return `*InvalidTagError`
- **AND** the card MUST be unchanged

#### Scenario: Patch with unknown card id

- **WHEN** `UpdateCard(b, "zzzzzz", any-patch)` is called and no card has id `zzzzzz`
- **THEN** the call MUST return `*CardNotFoundError`
- **AND** no card in `b.Cards` MUST be mutated

### Requirement: `CardPatch` distinguishes "absent" from "empty"

The package SHALL declare `CardPatch` with pointer fields for every patchable card attribute (at minimum: `Title *string`, `Description *string`, `Tags *[]string`, `Priority *string`). Pointer nil MUST mean "absent in this patch"; pointer non-nil MUST mean "explicit value, including the empty value". JSON encoding/decoding MUST honor this distinction via `omitempty` plus pointer presence.

#### Scenario: Unmarshalling JSON with absent key leaves pointer nil

- **WHEN** the JSON `{"title":"hi"}` is unmarshalled into a `CardPatch`
- **THEN** the resulting struct MUST have non-nil `Title` pointing to `"hi"`
- **AND** the resulting struct MUST have nil `Description`, `Tags`, `Priority`

#### Scenario: Unmarshalling JSON with empty value yields non-nil empty pointer

- **WHEN** the JSON `{"tags":[]}` is unmarshalled into a `CardPatch`
- **THEN** the resulting struct MUST have non-nil `Tags` pointing to an empty slice

### Requirement: `DeleteCard` removes a card from the board

The package SHALL expose `DeleteCard(b *Board, id string) error`
that removes the card whose `ID` equals `id` from `b.Cards`. The
helper MUST NOT persist (callers run `board.Save`). The helper
MUST NOT alter any other card's fields, including positional order
of the surviving cards (the deletion is a single-slice splice that
preserves the relative order of every other element).

If no card has the given `id`, the helper MUST return
`*CardNotFoundError` (the same typed error already returned by
`MoveCard` and `UpdateCard`) and MUST NOT mutate `b.Cards`.

The helper does NOT refresh any timestamp — the card is gone, so
there is no `UpdatedAt` to update; the surviving cards are not
modified.

#### Scenario: Successful delete removes the card and preserves order

- **WHEN** `b.Cards` contains three cards in order `[a, b, c]` and
  `DeleteCard(b, "b")` is called
- **THEN** the call MUST return `nil`
- **AND** `b.Cards` MUST contain exactly two cards in order
  `[a, c]`
- **AND** every field of the surviving cards `a` and `c` MUST be
  unchanged from before the call

#### Scenario: Unknown id returns *CardNotFoundError

- **WHEN** `b.Cards` does NOT contain any card with `ID = "zzzzzz"`
  and `DeleteCard(b, "zzzzzz")` is called
- **THEN** the call MUST return an error of type
  `*CardNotFoundError`
- **AND** the error's `ID` field MUST equal `"zzzzzz"`
- **AND** `b.Cards` MUST be byte-identical to its pre-call value
  (same length, same element order, same field values)

#### Scenario: Delete on a single-card board leaves an empty slice

- **WHEN** `b.Cards` contains exactly one card `a` and
  `DeleteCard(b, "a")` is called
- **THEN** the call MUST return `nil`
- **AND** `b.Cards` MUST have length `0`

#### Scenario: Delete does not touch board configuration

- **WHEN** `DeleteCard(b, <any-id>)` is called (success or
  failure)
- **THEN** `b.SchemaVersion`, `b.Board.Columns`, and
  `b.Board.Priorities` MUST be unchanged

#### Scenario: Delete is idempotent at the file layer (composed with Save)

- **WHEN** the caller composes `DeleteCard` followed by
  `Save(path, b)` for an existing card, then calls `DeleteCard`
  again for the same `id` on a freshly-loaded board
- **THEN** the second call MUST return `*CardNotFoundError`
- **AND** the file on disk after the second call MUST be
  byte-unchanged relative to its state after the first
  `Save` (no second write occurred because the caller did not
  reach `Save`)

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
