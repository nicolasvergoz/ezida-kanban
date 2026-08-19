## MODIFIED Requirements

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

### Requirement: Epic nesting is limited to one level

The system SHALL enforce that a card carrying a non-empty `epic` value MUST NOT itself be referenced as the `epic` of any other card. This makes reference cycles structurally unrepresentable rather than something to detect at load time — no graph traversal is required or permitted.

The rule SHALL be enforced symmetrically and before mutation. `CheckEpicTarget(b, a, childID, epicID)` MUST refuse, returning `*InvalidEpicError`, when:

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
