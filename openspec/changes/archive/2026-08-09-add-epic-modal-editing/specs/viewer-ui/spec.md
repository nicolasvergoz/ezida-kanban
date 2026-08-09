## RENAMED Requirements

- FROM: `### Requirement: The detail modal reports the epic relation read-only`
- TO: `### Requirement: The detail modal reports and edits the epic relation`

## MODIFIED Requirements

### Requirement: The detail modal reports and edits the epic relation

The card-detail modal SHALL report a card's epic relation and SHALL offer the controls that change it.

The modal MUST render an `Epic` section on **every** card. For a card carrying an `epic`, that section names the parent — the parent's colored chip and its id — alongside a reassign control and a detach control. For a card carrying none, it renders an attach affordance in the same place. The always-present section is what makes a first relation reachable from a board that has none; the board surface itself remains unchanged for a board without epics.

For a card referenced as an epic, the modal MUST additionally render a `Children` section listing each child with its title and column, plus the progress bar, the `done/total` counter, an add-a-child control, and a per-row remove control. A card MUST NEVER render both an assigned parent and a children list, because one-level nesting makes that state unrepresentable — and the attach affordance MUST NOT be offered on a card that has children.

Every relation write MUST be a single `PATCH /api/cards/{childId}` setting `epic` to the target id or to the empty string. The parent card MUST NEVER be written by the client to establish or break a relation.

#### Scenario: A child shows its parent with controls

- **WHEN** the modal opens on a card carrying `epic: "rl4m9x"`
- **THEN** it renders a section naming card `rl4m9x`, with that card's chip and id
- **AND** that section exposes a reassign control and a detach control
- **AND** it renders no children section

#### Scenario: A parent shows its children and progress

- **WHEN** the modal opens on a card referenced by three others, one of which is in a terminal column
- **THEN** it renders a list of those three cards with their titles and columns
- **AND** it renders a progress bar and the counter `1/3`
- **AND** it renders no assigned-parent row

#### Scenario: Children are listed in board order

- **WHEN** an epic's children appear in the `/api/board` `cards` array in the order `[c, a, b]`
- **THEN** the modal lists them in that order

#### Scenario: An unrelated card offers the attach affordance

- **WHEN** the modal opens on a card that carries no `epic` and is referenced by none
- **THEN** the modal MUST render the `Epic` section with an attach affordance
- **AND** it MUST render no children section

#### Scenario: Attaching commits one PATCH on the child

- **WHEN** the user attaches the open card to epic `rl4m9x`
- **THEN** exactly one `PATCH /api/cards/<open card id>` with body `{"epic":"rl4m9x"}` is issued
- **AND** no request is issued against `rl4m9x`
- **AND** the board is refetched and the modal shows `rl4m9x` as the parent

#### Scenario: Detaching clears the field

- **WHEN** the user activates detach on a card carrying an epic
- **THEN** exactly one `PATCH` with body `{"epic":""}` is issued for that card
- **AND** after the refetch the modal shows the attach affordance

#### Scenario: Removing a child from the parent side patches the child

- **WHEN** the user activates the remove control on the row for child `aaaaa1`
- **THEN** exactly one `PATCH /api/cards/aaaaa1` with body `{"epic":""}` is issued
- **AND** the child disappears from the list after the refetch
- **AND** the counter and the progress bar recompute

#### Scenario: A parent is never offered an epic of its own

- **WHEN** the modal opens on a card that has at least one child
- **THEN** it MUST NOT render an attach affordance for that card

## ADDED Requirements

### Requirement: A card-search combobox picks a related card

The viewer SHALL provide a card-search combobox used wherever a card must name another card. In this change it is used twice — attaching or reassigning from the child side, and adding a child from the parent side — and it is written to be reused for future relations.

The control MUST filter the cards already held by the client, with no additional request. A card matches when the query is a case-insensitive substring of its title or a prefix of its id. Results MUST show enough to disambiguate two cards with the same title: the title, the id, and the column.

The candidate list MUST exclude every card the server would refuse, which is not the same set on both sides of the relation. Picking an **epic** for the open card, the list MUST exclude the open card itself and every card that already carries an `epic` — a card with children is the normal target and MUST remain listed. Picking a **child** for the open card, the list MUST exclude the open card itself, every card that already has children of its own, and every card already attached to this epic; a card belonging to a different epic MUST remain listed, since reassignment is legal.

Client-side exclusion is a courtesy, not the authority — a selection the server refuses MUST surface as an error rather than be treated as impossible.

Keyboard behaviour MUST be complete: typing filters, `ArrowDown`/`ArrowUp` move the highlight, `Enter` commits the highlighted candidate, `Escape` closes the picker. While the picker is open, `Escape` MUST NOT reach the modal. A click outside the picker MUST close it without committing.

The control MUST use combobox semantics: `role="combobox"` on the input with `aria-expanded`, and `aria-activedescendant` naming the highlighted option in an associated `role="listbox"`.

#### Scenario: Typing filters by title

- **WHEN** the user types `sche` into the picker
- **THEN** only cards whose title contains `sche`, case-insensitively, remain listed

#### Scenario: Typing filters by id prefix

- **WHEN** the user types `rl4` into the picker
- **THEN** the card with id `rl4m9x` is listed

#### Scenario: Invalid epic targets are absent

- **WHEN** the picker opens on card `X` to choose an epic for it
- **THEN** `X` itself MUST NOT be listed
- **AND** no card carrying an `epic` MUST be listed
- **AND** a card that already has children MUST be listed

#### Scenario: Invalid children are absent

- **WHEN** the picker opens on epic `E` to choose a child for it
- **THEN** `E` itself MUST NOT be listed
- **AND** no card having children of its own MUST be listed
- **AND** no card already attached to `E` MUST be listed

#### Scenario: Enter commits the highlighted candidate

- **WHEN** the user presses `ArrowDown` twice and then `Enter`
- **THEN** the third listed candidate is committed as the relation target
- **AND** the picker closes

#### Scenario: Escape closes the picker, not the modal

- **WHEN** the picker is open and the user presses `Escape`
- **THEN** the picker closes
- **AND** the modal MUST remain open

#### Scenario: Clicking outside closes without committing

- **WHEN** the picker is open and the user clicks elsewhere in the modal
- **THEN** the picker closes
- **AND** no `PATCH` is issued

#### Scenario: A server refusal is reported, not swallowed

- **WHEN** a committed selection returns `400 INVALID_EPIC`
- **THEN** the modal MUST display the server's message
- **AND** the relation MUST remain as it was

### Requirement: The modal exposes the epic palette as swatches

On a card that has children, the modal SHALL render the epic color palette as a row of swatches inside the `Children` section, plus a control that clears the color.

Each swatch MUST send the palette entry's **hex** value in `PATCH {"color":"#rrggbb"}`. Palette names travel only as accessible labels and tooltips; the wire accepts hex alone.

The swatch matching the card's current color MUST be marked selected. A card carrying a hex that is not in the palette MUST render that value as an additional, selected swatch rather than showing no selection, so a hand-edited color is visible and is not overwritten by accident.

The row MUST NOT be rendered on a card without children: color has no rendering consequence there, and a control with an invisible effect is worse than its absence.

#### Scenario: The palette is offered on an epic

- **WHEN** the modal opens on a card with at least one child
- **THEN** it renders one swatch per palette entry, in palette order
- **AND** the swatch whose hex equals the card's `color` is marked selected

#### Scenario: Clicking a swatch patches the hex

- **WHEN** the user clicks the `emerald` swatch
- **THEN** exactly one `PATCH` with body `{"color":"#10b981"}` is issued for that card
- **AND** after the refetch the chips on that epic's children render in the new color

#### Scenario: An off-palette color is represented

- **WHEN** the modal opens on an epic whose `color` is `#123456`
- **THEN** an additional swatch showing `#123456` is rendered and marked selected

#### Scenario: Clearing removes the color

- **WHEN** the user activates the clear control
- **THEN** exactly one `PATCH` with body `{"color":""}` is issued

#### Scenario: No palette on a childless card

- **WHEN** the modal opens on a card with no children
- **THEN** no swatch row MUST be rendered

### Requirement: The modal reports mutation failures instead of logging them

The modal SHALL render an error region carrying the `error.message` of the last failed mutation originating from it. The message MUST be cleared by the next successful mutation and when the modal closes.

A failure the UI has handled — any `4xx` carrying the standard error envelope — MUST NOT be written to the browser console. Unexpected failures — a `5xx`, a network error, a body that is not the envelope — MUST still be logged, since nothing else records them.

This applies to every mutation issued by the modal, not only the epic ones; the epic controls are simply the first whose failure carries information a user needs.

#### Scenario: An invalid epic target shows its message

- **WHEN** a relation commit returns `400 INVALID_EPIC` with message `board: epic "zzzzzz" is invalid: no card on this board carries that id`
- **THEN** the modal renders that message
- **AND** nothing is written to the browser console

#### Scenario: The message clears on the next success

- **WHEN** an error message is displayed and a subsequent mutation succeeds
- **THEN** the message MUST no longer be rendered

#### Scenario: The message clears on close

- **WHEN** an error message is displayed and the modal is closed and reopened on the same card
- **THEN** no message MUST be rendered

#### Scenario: An unexpected failure still reaches the console

- **WHEN** a mutation returns `500`
- **THEN** the modal renders a message
- **AND** the failure is logged to the browser console
