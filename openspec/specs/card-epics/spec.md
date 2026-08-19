# Card Epics Specification

## Purpose

The epic relation between cards: the `epic` and `color` fields, the one-level nesting rule that makes cycles unrepresentable, the named color palette and its collision-free assignment, the derived children/progress values, and `ezida colors`. Per-command flag behavior lives in the command capabilities (card-writing, board-config, card-reading).

## Requirements

### Requirement: A card MAY declare a parent epic

A `[[cards]]` entry SHALL accept an OPTIONAL `epic` key whose value is the six-character id of another card on the same board. The field MUST be modeled as `Card.Epic string` with TOML tag `epic,omitempty`, so an unset value is never written back to disk.

The relation is purely documentary. Declaring an epic MUST NOT restrict, refuse, or warn about any subsequent operation on either card — a child may move to any column, a parent may move to any column, and either may be edited or deleted independently of the other.

#### Scenario: Round-trip preserves the epic reference

- **WHEN** a `kanban.toml` containing a card with `epic = 'rl4m9x'` is loaded and saved without modification
- **THEN** the saved file MUST contain the same `epic = 'rl4m9x'` on the same card

#### Scenario: Absent epic round-trips as absent

- **WHEN** a card without an `epic` key is loaded and saved without modification
- **THEN** the saved card block MUST NOT contain an `epic` key

#### Scenario: A child moves freely between columns

- **WHEN** a card with `epic = 'rl4m9x'` is moved to any declared column
- **THEN** the move MUST succeed with exit code `0`
- **AND** no warning MUST be emitted about the epic relation

### Requirement: Epic nesting is limited to one level

The system SHALL enforce that a card carrying a non-empty `epic` value MUST NOT itself be referenced as the `epic` of any other card. This makes reference cycles structurally unrepresentable rather than something to detect at load time — no graph traversal is required or permitted.

The rule SHALL be enforced symmetrically and before mutation. `CheckEpicTarget(b, childID, epicID)` MUST refuse, returning `*InvalidEpicError`, when:

- `epicID` equals `childID` — a card cannot be its own epic;
- no card on the board carries `epicID`;
- the card named by `epicID` itself carries a non-empty `epic`;
- the card named by `childID` is the epic of at least one other card, **live or archived**.

The fourth rule is new. Without it, giving an epic a parent of its own produces a board whose children sit two levels deep, refused only afterwards by the whole-board `Validate` — a board-level report for what is a single invalid argument. Callers MUST be able to reject the operation before writing anything, and MUST receive an error naming the offending id.

The fourth rule counts archived children. A card whose every child has been archived MUST still be refused an `epic` of its own: the archived child would otherwise create a two-level nest the moment it is restored, and the refusal would then surface as a whole-board validation failure on the restore — far from the edit that caused it — instead of at the edit itself. The archive is consulted for this rule only; a nil or absent archive reduces it to the live-only check.

`Validate` keeps the load-time rule unchanged; it is the guard for boards edited outside the tool. It reads `kanban.toml` alone, so it neither sees nor needs the archive.

#### Scenario: Two-level chain is rejected

- **WHEN** `Validate` runs on a board where card `A` has `epic = 'B'` and card `B` has `epic = 'C'`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST name card `B` as violating the one-level rule

#### Scenario: Many children on one parent is legal

- **WHEN** `Validate` runs on a board where six cards all declare `epic = 'rl4m9x'` and card `rl4m9x` declares no epic
- **THEN** it MUST return `nil`

#### Scenario: A card with children cannot be given an epic

- **WHEN** `CheckEpicTarget(b, a, "rl4m9x", "aaaaa1")` is called and at least one card on `b` declares `epic = 'rl4m9x'`
- **THEN** it MUST return `*InvalidEpicError`
- **AND** the error's `Reason` MUST explain that the card has children of its own and that nesting is limited to one level
- **AND** the board MUST be unmodified

#### Scenario: A childless card may still be given an epic

- **WHEN** `CheckEpicTarget(b, a, "loose1", "rl4m9x")` is called, no card in `b` or `a` declares `epic = 'loose1'`, and `rl4m9x` carries no epic
- **THEN** it MUST return `nil`

#### Scenario: `ezida edit --epic` refuses to nest a parent

- **WHEN** `ezida edit rl4m9x --epic=aaaaa1` is invoked and `rl4m9x` has at least one child
- **THEN** the command MUST exit non-zero with the `INVALID_EPIC` code
- **AND** `kanban.toml` MUST be byte-unchanged

#### Scenario: A card whose children are all archived is still refused a parent

- **WHEN** `CheckEpicTarget(b, a, "P", "aaaaa1")` is called, no live card declares `epic = 'P'`, but at least one archived card does
- **THEN** it MUST return `*InvalidEpicError`
- **AND** the error's `Reason` MUST explain the one-level nesting rule

#### Scenario: A nil archive reduces the nesting guard to the live check

- **WHEN** `CheckEpicTarget` is called with a nil archive
- **THEN** its result MUST equal the result the live-only guard produced before archived children were considered

### Requirement: A card MAY carry a color

A `[[cards]]` entry SHALL accept an OPTIONAL `color` key holding a CSS hex string matching `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`. The field MUST be modeled as `Card.Color string` with TOML tag `color,omitempty`.

The on-disk representation MUST always be a literal hex value. Named palette entries are a convenience of the CLI and the viewer; the file format MUST remain readable by a consumer that has no knowledge of the named set.

The color has no meaning of its own — it is the identity a parent lends to its children in presentation surfaces. A card with a `color` but no children is legal and MUST NOT be reported as a violation.

#### Scenario: Hex color round-trips

- **WHEN** a card with `color = '#8b5cf6'` is loaded and saved without modification
- **THEN** the saved card MUST contain `color = '#8b5cf6'`

#### Scenario: Named color is never written to disk

- **WHEN** `ezida edit rl4m9x --color=violet` is invoked
- **THEN** the saved card MUST contain the hex value bound to `violet` in the palette
- **AND** the saved card MUST NOT contain the literal string `violet`

#### Scenario: Malformed color rejected

- **WHEN** `Validate` runs on a board whose card has `color = 'violet'` or `color = '#12'`
- **THEN** it MUST return a `*ValidationError` naming the offending card and value

### Requirement: A named color palette with collision-free assignment

The `board` package SHALL expose an ordered palette of named colors. The set is:

| Order | Name | Hex |
|-------|---------|-----------|
| 1 | `violet` | `#8b5cf6` |
| 2 | `emerald` | `#10b981` |
| 3 | `orange` | `#f97316` |
| 4 | `blue` | `#3b82f6` |
| 5 | `pink` | `#ec4899` |
| 6 | `lime` | `#84cc16` |
| 7 | `cyan` | `#06b6d4` |
| 8 | `fuchsia` | `#d946ef` |

The order is deliberately chromatic-distance ordering, not hue ordering, so that consecutively assigned colors are visually distinguishable at chip size.

The palette MUST NOT contain the three default priority colors (`#ef4444`, `#f59e0b`, `#22c55e`), so that an epic chip is never confused with a priority indicator.

When a card acquires an epic role and carries no explicit color, the system SHALL assign the palette color that is **least used** among cards on the board that already carry a color, breaking ties by palette order. A modulo-of-count strategy MUST NOT be used, because deleting a card would cause the next assignment to collide with a color already in use.

#### Scenario: First epic receives the first palette color

- **WHEN** a card first acquires children on a board where no card carries a color
- **THEN** its assigned color MUST equal `#8b5cf6`

#### Scenario: Assignment prefers an unused color

- **WHEN** a new epic is created on a board where `#8b5cf6` and `#10b981` are each used once and the six remaining palette colors are unused
- **THEN** the assigned color MUST equal `#f97316`

#### Scenario: Assignment reuses the least-used color once the palette is exhausted

- **WHEN** a ninth epic is created on a board where each of the eight palette colors is used exactly once
- **THEN** the assigned color MUST equal `#8b5cf6` (the earliest of the tied least-used colors)

#### Scenario: Deletion does not cause a collision

- **WHEN** three epics hold `#8b5cf6`, `#10b981`, `#f97316`, the `#10b981` epic is deleted, and a new epic is created
- **THEN** the assigned color MUST equal `#10b981`

#### Scenario: An explicit color survives assignment

- **WHEN** a card already carries `color = '#7c3aed'` and acquires a child
- **THEN** its color MUST remain `#7c3aed`

### Requirement: `ezida colors` reports the palette and its holders

`ezida colors` SHALL list every palette entry with its name, hex value, and the epic currently holding it (id and title), or a free marker when unheld. The command MUST NOT mutate the board.

JSON output MUST follow:
```json
{
  "colors": [
    {"name": "violet", "hex": "#8b5cf6", "held_by": {"id": "rl4m9x", "title": "Card relations"}},
    {"name": "emerald", "hex": "#10b981", "held_by": null}
  ]
}
```

#### Scenario: Listing reports holders

- **WHEN** `ezida colors --json` is invoked on a board where one epic holds `#8b5cf6`
- **THEN** the `violet` entry's `held_by.id` MUST equal that epic's id
- **AND** every other entry's `held_by` MUST be `null`

#### Scenario: Listing includes off-palette colors in use

- **WHEN** `ezida colors --json` is invoked on a board where an epic holds `#7c3aed`, which is not in the palette
- **THEN** the output MUST include an additional entry with `hex` equal to `#7c3aed` and a `name` of `null`

### Requirement: Epic progress is derived, never stored

The system SHALL derive, for any card referenced as an `epic` by at least one other card:

- `children`: the ids of every card whose `epic` equals this card's id — live cards in board file order, followed by archived cards in archive file order.
- `done`: the count of those children that count as done (see below).
- `total`: the count of those children.

Archived children SHALL be counted. Archiving is not deletion — it is the operation that preserves the record — so filing away completed work MUST NOT make an epic report less progress than it did before.

A live child counts toward `done` when its `column` is a terminal column. An archived child counts toward `done` when the `column` recorded in the archive — the column it occupied when archived — is a terminal column **at read time**. Done-ness is therefore resolved against the board's current configuration for both, never stored.

These values MUST NOT be persisted to `kanban.toml` or to the archive file. A card referenced by no other card, live or archived, MUST NOT expose them at all.

A board with no terminal columns MUST report `done = 0` for every epic. This is a truthful reading of the board's configuration, not an error condition, and MUST NOT produce a warning.

Because done-ness is resolved at read time, deleting the column an archived child came from, or clearing that column's terminal marker, MUST cause the child to stop counting toward `done` while continuing to count toward `total`. This follows from the same rule applied to live cards and is a known, documented consequence rather than a special case.

#### Scenario: Progress counts children in terminal columns

- **WHEN** an epic has three children, one of which sits in a terminal column
- **THEN** its derived `done` MUST equal `1` and its `total` MUST equal `3`

#### Scenario: Children preserve file order

- **WHEN** an epic's children appear in `kanban.toml` in the order `[c, a, b]`
- **THEN** the derived `children` slice MUST equal `[c, a, b]`

#### Scenario: No terminal columns yields zero progress

- **WHEN** an epic has three children and no column carries the terminal marker
- **THEN** its derived `done` MUST equal `0`
- **AND** the command MUST exit `0` with no warning

#### Scenario: Derived values are absent from the saved file

- **WHEN** a board containing an epic with children is loaded and saved
- **THEN** the saved file MUST NOT contain any `children`, `done`, `total`, or `progress` key

#### Scenario: Archiving a completed child does not lower the count

- **WHEN** an epic has four children, all four in a terminal column, and three of them are archived
- **THEN** its derived `done` MUST equal `4` and its `total` MUST equal `4`
- **AND** the values MUST be identical to those derived before the three were archived

#### Scenario: An archived child from a non-terminal column counts only toward total

- **WHEN** an epic has one live child in a terminal column and one child archived from a non-terminal column
- **THEN** its derived `done` MUST equal `1` and its `total` MUST equal `2`

#### Scenario: Archived children appear in the derived children list

- **WHEN** an epic has one live child and two archived children
- **THEN** the derived `children` MUST contain all three
- **AND** the live child MUST precede both archived children

#### Scenario: A card whose children are all archived is still an epic

- **WHEN** every card referencing a live card as its `epic` has been archived
- **THEN** that live card MUST still be treated as an epic
- **AND** it MUST still expose `children`, `done`, and `total`

#### Scenario: Removing the column an archived child came from drops it from done

- **WHEN** an epic has one child archived from a terminal column, and that column is subsequently deleted from `[board].columns`
- **THEN** the child MUST stop counting toward `done`
- **AND** it MUST continue counting toward `total`

#### Scenario: A board with no archive derives exactly as before

- **WHEN** progress is derived for any epic on a board with no archive file
- **THEN** the result MUST equal the result the same board produced before archived children were counted
