## MODIFIED Requirements

### Requirement: `ezida list` and its filters

`ezida list` SHALL print every card in the board by default, preserving
file order. Filters:

- `--column=<name>`: keep only cards whose `column` matches exactly.
- `--title-contains=<substr>`: keep only cards whose `title` contains
  `<substr>` (case-insensitive).
- `--tag=<tag>`: keep only cards that have `<tag>` in their `tags` array.
- `--priority=<priority>`: keep only cards whose `priority` matches
  exactly. Cards without a `priority` are excluded by this filter.
- `--epic=<id>`: keep only the card whose `id` equals `<id>` and the
  cards whose `epic` equals `<id>`. The parent is included so that
  scoping to an epic never hides the epic itself. An id matching no card
  is rejected with `INVALID_FILTER`.

Multiple filters MUST be AND-combined.

Two flags select the archive:

- `--include-archived`: append archived cards to the live results.
- `--archived-only`: return archived cards instead of live ones.

The two MUST NOT be combined; supplying both MUST be rejected with
`MUTUALLY_EXCLUSIVE_FLAGS` and exit code `1`. When neither is supplied, the
archive MUST NOT be read and the output MUST be byte-identical to the output
the same invocation produced before archiving existed.

With `--include-archived`, live cards MUST be listed first in board file order,
followed by archived cards in archive file order. Results MUST NOT be
interleaved by date: file order is the board's only ordering concept.

Filters apply to both sets. With `--archived-only`, the validity check for
`--column`, `--priority` and `--epic` MUST widen to accept any value present
among archived cards, because an archived card can reference a column,
priority or epic the board no longer declares.

JSON output MUST follow:
```json
{
  "cards": [
    {
      "id": "a3f2k9",
      "title": "Refactor auth",
      "column": "todo",
      "priority": "high",
      "tags": ["security"],
      "epic": "rl4m9x",
      "created_at": "2026-05-20T14:30:00Z",
      "updated_at": "2026-05-20T14:30:00Z"
    }
  ]
}
```
The `description` field MUST NOT appear in `list --json` output. The
`epic` and `color` fields MUST be omitted when unset. An `archived_at` field
MUST be present on archived cards and MUST be **absent** — not null, not a zero
timestamp — on live cards.

Text output MUST be an aligned table with a header row:
```
ID      COLUMN   PRI   TITLE              TAGS
a3f2k9  todo     high  Refactor auth      security,tech-debt
b7m1p4  todo     -     Update README      -
```
Missing priority is rendered as `-`. Empty tags are rendered as `-`. An
`ARCHIVED` column MUST be appended when, and only when, one of the two archive
flags is supplied; its value is the archive date as `YYYY-MM-DD`, or `-` for a
live card.

#### Scenario: No filters returns every card

- **WHEN** `ezida list --json` is invoked against a board with 11 cards
- **THEN** the `cards` array length equals 11
- **AND** the IDs appear in the same order as in `kanban.toml`

#### Scenario: AND-combined filters

- **WHEN** `ezida list --column=todo --tag=security` is invoked
- **THEN** every returned card has `column = "todo"` AND `"security"` in
  `tags`

#### Scenario: Case-insensitive title substring

- **WHEN** `ezida list --title-contains=AUTH` is invoked against a board
  with a card titled `"Refactor auth module"`
- **THEN** that card appears in the output

#### Scenario: Description omitted in list JSON

- **WHEN** `ezida list --json` is invoked
- **THEN** no card object in `cards` contains a `description` key

#### Scenario: Unknown column filter is a user error

- **WHEN** `ezida list --column=ghost` is invoked against a board whose
  `columns` does not include `ghost`
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `INVALID_FILTER`

#### Scenario: Epic filter includes the parent

- **WHEN** `ezida list --epic=rl4m9x --json` is invoked against a board
  where card `rl4m9x` exists and three cards carry `epic = "rl4m9x"`
- **THEN** the `cards` array length equals 4
- **AND** it MUST include card `rl4m9x`

#### Scenario: Unknown epic filter is a user error

- **WHEN** `ezida list --epic=zzzzzz` is invoked and no card `zzzzzz`
  exists
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `INVALID_FILTER`

#### Scenario: Epic filter combines with other filters

- **WHEN** `ezida list --epic=rl4m9x --column=backlog` is invoked
- **THEN** every returned card MUST be in `backlog` AND MUST either
  carry `epic = "rl4m9x"` or be card `rl4m9x` itself

#### Scenario: Default output ignores the archive entirely

- **WHEN** `ezida list` is invoked against a board that has an archive file
- **THEN** stdout MUST be byte-identical to the output of the same invocation
  against the same board with the archive file removed

#### Scenario: Live cards precede archived cards

- **WHEN** `ezida list --include-archived --json` is invoked
- **THEN** every live card MUST appear before every archived card
- **AND** the live cards MUST be in board file order
- **AND** the archived cards MUST be in archive file order

#### Scenario: `archived_at` is absent for live cards

- **WHEN** `ezida list --include-archived --json` is invoked
- **THEN** no live card object MUST contain an `archived_at` key
- **AND** every archived card object MUST contain one

#### Scenario: Both archive flags is a user error

- **WHEN** `ezida list --include-archived --archived-only` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `MUTUALLY_EXCLUSIVE_FLAGS`

#### Scenario: `--archived-only` accepts a column the board dropped

- **WHEN** `ezida list --archived-only --column=review` is invoked and `review`
  exists only among archived cards
- **THEN** the process exits with code `0`
- **AND** the matching archived cards are returned
