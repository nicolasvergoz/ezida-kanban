## MODIFIED Requirements

### Requirement: `ezida get` reports a single card with full description

`ezida get <id>` SHALL look up the card by exact ID and print its full
detail.

When the card carries an `epic`, the output MUST report the parent's id
and title. When the card is referenced as the `epic` of other cards, the
output MUST instead report its children with their columns, plus the
derived done/total counts. A card MUST NEVER report both, because
one-level nesting makes that state unrepresentable.

The command SHALL load the archive alongside the board so that archived
children are reported. Archived children MUST be **listed**, not merely
counted: a `progress` total that does not match the visible child list
would misrepresent the epic more than omitting them did. Each archived
child MUST be marked as archived — an `"archived": true` key on its
entry in JSON mode, and a visible marker on its line in text mode — so
the reader can tell why a child is not in any column on the board.

Live children MUST be listed before archived ones. A missing archive
file MUST be treated as an empty archive, leaving the output identical
to a board that has never archived anything.

JSON output MUST follow:
```json
{
  "card": {
    "id": "a3f2k9",
    "title": "Refactor auth",
    "column": "todo",
    "priority": "high",
    "tags": ["security"],
    "description": "Move from session-based to JWT.\nCheck token expiry handling.\n",
    "epic": {"id": "rl4m9x", "title": "Card relations"},
    "created_at": "2026-05-20T14:30:00Z",
    "updated_at": "2026-05-20T14:30:00Z"
  }
}
```

For a parent card, `epic` is omitted and the card object instead carries:
```json
{
  "color": "#8b5cf6",
  "children": [
    {"id": "f20wbo", "title": "Card dependencies", "column": "backlog"},
    {"id": "q7t6z2", "title": "Card colors", "column": "done", "archived": true}
  ],
  "progress": {"done": 1, "total": 3}
}
```
The `archived` key MUST be omitted entirely for a live child, never
emitted as `false`, so a board with no archive produces byte-identical
output to before this behaviour existed.

Text output MUST be a key:value block:
```
ID:         a3f2k9
Title:      Refactor auth module
Column:     todo
Priority:   high
Tags:       security, tech-debt
Created:    2026-05-20T14:30:00Z
Updated:    2026-05-20T14:30:00Z

Description:
Move from session-based to JWT.
Check token expiry handling.
```

#### Scenario: Get returns full card with description

- **WHEN** `ezida get a3f2k9 --json` is invoked and card `a3f2k9` exists
  with a multi-line description
- **THEN** the `card.description` field equals the file's description
  byte-for-byte (after TOML unescaping)

#### Scenario: A child reports its parent

- **WHEN** `ezida get f20wbo --json` is invoked on a card carrying
  `epic = "rl4m9x"`
- **THEN** `card.epic.id` MUST equal `"rl4m9x"`
- **AND** `card.epic.title` MUST equal that card's title
- **AND** `card.children` and `card.progress` MUST be absent

#### Scenario: A parent reports its children and progress

- **WHEN** `ezida get rl4m9x --json` is invoked on a card referenced by
  three others, one of which sits in a terminal column
- **THEN** `card.children` MUST list the three cards in board file order
  with their ids, titles, and columns
- **AND** `card.progress` MUST equal `{"done": 1, "total": 3}`
- **AND** `card.epic` MUST be absent

#### Scenario: An unrelated card reports neither

- **WHEN** `ezida get b7m1p4 --json` is invoked on a card that carries no
  `epic` and is referenced by none
- **THEN** `card.epic`, `card.children`, and `card.progress` MUST all be
  absent

#### Scenario: Text mode reports the relation

- **WHEN** `ezida get rl4m9x` is invoked on a parent card in text mode
- **THEN** stdout MUST include a line naming each child card

#### Scenario: Archived children are listed and counted

- **WHEN** `ezida get rl4m9x --json` is invoked on an epic with one live
  child and two archived children
- **THEN** `card.children` MUST have length 3
- **AND** the live child MUST appear before both archived children
- **AND** `card.progress.total` MUST equal `3`

#### Scenario: An archived child is marked in JSON

- **WHEN** `ezida get rl4m9x --json` is invoked on an epic with an
  archived child
- **THEN** that child's entry MUST contain `"archived": true`
- **AND** no live child's entry MUST contain an `archived` key at all

#### Scenario: An archived child is marked in text mode

- **WHEN** `ezida get rl4m9x` is invoked in text mode on an epic with an
  archived child
- **THEN** that child's line MUST carry a visible marker distinguishing
  it from a live child

#### Scenario: A board with no archive is unchanged

- **WHEN** `ezida get rl4m9x --json` is invoked on a board with no
  archive file
- **THEN** stdout MUST be byte-identical to the output the same
  invocation produced before archived children were reported

#### Scenario: An epic whose children are all archived still reports them

- **WHEN** `ezida get rl4m9x --json` is invoked on a card whose every
  child has been archived
- **THEN** `card.children` MUST list them
- **AND** `card.progress` MUST be present
- **AND** `card.epic` MUST be absent
