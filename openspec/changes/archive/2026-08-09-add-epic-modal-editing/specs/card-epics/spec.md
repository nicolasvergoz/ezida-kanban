## MODIFIED Requirements

### Requirement: Epic nesting is limited to one level

The system SHALL enforce that a card carrying a non-empty `epic` value MUST NOT itself be referenced as the `epic` of any other card. This makes reference cycles structurally unrepresentable rather than something to detect at load time — no graph traversal is required or permitted.

The rule SHALL be enforced symmetrically and before mutation. `CheckEpicTarget(b, childID, epicID)` MUST refuse, returning `*InvalidEpicError`, when:

- `epicID` equals `childID` — a card cannot be its own epic;
- no card on the board carries `epicID`;
- the card named by `epicID` itself carries a non-empty `epic`;
- the card named by `childID` is the epic of at least one other card.

The fourth rule is new. Without it, giving an epic a parent of its own produces a board whose children sit two levels deep, refused only afterwards by the whole-board `Validate` — a board-level report for what is a single invalid argument. Callers MUST be able to reject the operation before writing anything, and MUST receive an error naming the offending id.

`Validate` keeps the load-time rule unchanged; it is the guard for boards edited outside the tool.

#### Scenario: Two-level chain is rejected

- **WHEN** `Validate` runs on a board where card `A` has `epic = 'B'` and card `B` has `epic = 'C'`
- **THEN** it MUST return a `*ValidationError`
- **AND** the error MUST name card `B` as violating the one-level rule

#### Scenario: Many children on one parent is legal

- **WHEN** `Validate` runs on a board where six cards all declare `epic = 'rl4m9x'` and card `rl4m9x` declares no epic
- **THEN** it MUST return `nil`

#### Scenario: A card with children cannot be given an epic

- **WHEN** `CheckEpicTarget(b, "rl4m9x", "aaaaa1")` is called and at least one card on `b` declares `epic = 'rl4m9x'`
- **THEN** it MUST return `*InvalidEpicError`
- **AND** the error's `Reason` MUST explain that the card has children of its own and that nesting is limited to one level
- **AND** the board MUST be unmodified

#### Scenario: A childless card may still be given an epic

- **WHEN** `CheckEpicTarget(b, "loose1", "rl4m9x")` is called, no card declares `epic = 'loose1'`, and `rl4m9x` carries no epic
- **THEN** it MUST return `nil`

#### Scenario: `ezida edit --epic` refuses to nest a parent

- **WHEN** `ezida edit rl4m9x --epic=aaaaa1` is invoked and `rl4m9x` has at least one child
- **THEN** the command MUST exit non-zero with the `INVALID_EPIC` code
- **AND** `kanban.toml` MUST be byte-unchanged
