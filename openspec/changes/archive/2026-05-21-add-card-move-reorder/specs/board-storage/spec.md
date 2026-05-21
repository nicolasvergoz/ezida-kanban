## ADDED Requirements

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

## MODIFIED Requirements

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
