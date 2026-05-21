## ADDED Requirements

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
