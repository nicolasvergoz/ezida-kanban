# Viewer UI Specification (delta)

## ADDED Requirements

### Requirement: A card belonging to an epic renders a colored chip

A card whose `epic` names another card on the board SHALL render a chip in the existing `.card-foot` row, positioned as the first element after the priority pill and before every tag chip. The chip MUST show the four-square epic glyph followed by the parent card's title.

The chip MUST be colored from the parent's `color`. Tag chips MUST remain neutral. Color alone therefore distinguishes the two, so both MAY keep the same pill shape — no separate shape vocabulary is introduced.

The chip MUST derive its background, border, and text color from the stored hex by mixing it toward the theme's current `--text`, never by using the hex directly as a text color. A single stored value MUST render legibly in both light and dark themes.

The chip MUST be constrained by a `max-width` and truncate with an ellipsis, and MUST carry the untruncated parent title in a `title` attribute.

A card whose `epic` names a card that is not present in the payload MUST render no chip rather than an empty or broken one.

#### Scenario: Chip renders with the parent's title and color

- **WHEN** the board contains card `f20wbo` with `epic: "rl4m9x"` and card `rl4m9x` with `title: "Card relations"` and `color: "#8b5cf6"`
- **THEN** card `f20wbo` renders a chip labelled `Card relations`
- **AND** the chip's color is derived from `#8b5cf6`

#### Scenario: Chip precedes the tags

- **WHEN** a card carries both an `epic` and two tags
- **THEN** the epic chip MUST appear before both tag chips in DOM order within `.card-foot`

#### Scenario: Priority pill keeps its position

- **WHEN** a card carries a priority and an epic
- **THEN** the priority pill MUST remain the first element of `.card-foot`
- **AND** the epic chip MUST follow it

#### Scenario: Tags stay neutral

- **WHEN** a card renders both an epic chip and tag chips
- **THEN** the tag chips MUST use the existing neutral tag styling, unchanged

#### Scenario: A card without an epic renders no chip

- **WHEN** a card carries no `epic` field
- **THEN** its `.card-foot` MUST be identical to the pre-epic rendering

#### Scenario: Long parent titles truncate

- **WHEN** the parent's title exceeds the chip's `max-width`
- **THEN** the visible label MUST be truncated with an ellipsis
- **AND** the element's `title` attribute MUST hold the full parent title

#### Scenario: A dangling epic reference renders nothing

- **WHEN** a card carries `epic: "zzzzzz"` and no card with that id is present in the payload
- **THEN** no chip MUST be rendered for that card
- **AND** the page MUST NOT throw

### Requirement: A card referenced as an epic renders as a parent

A card referenced by the `epic` field of at least one other card SHALL render three additional signals: the four-square epic glyph immediately before its title, a border tinted from its own `color`, and a progress bar with a `done/total` counter.

`total` is the count of cards whose `epic` equals this card's id. `done` is the subset of those whose `column` appears in the board's `done_columns`. Both MUST be computed in the client from the `/api/board` payload; neither is read from the wire.

All three signals MUST be conditional on the card actually having children. A card carrying a `color` but referenced by nobody MUST render exactly as it does today.

The counter MUST use the monospace face with tabular numerals so counters align down a column.

#### Scenario: A parent renders the glyph, tint, and bar

- **WHEN** three cards carry `epic: "rl4m9x"` and card `rl4m9x` carries `color: "#8b5cf6"`
- **THEN** card `rl4m9x` renders the epic glyph before its title
- **AND** its border is tinted from `#8b5cf6`
- **AND** it renders a progress bar and counter

#### Scenario: Progress counts children in terminal columns

- **WHEN** an epic has three children and exactly one sits in a column listed in `done_columns`
- **THEN** the counter reads `1/3`
- **AND** the bar is filled to one third

#### Scenario: No terminal columns yields a zero bar

- **WHEN** an epic has three children and `done_columns` is `[]`
- **THEN** the counter reads `0/3`
- **AND** the bar renders empty rather than absent

#### Scenario: A colored card with no children is unchanged

- **WHEN** a card carries a `color` but no other card references it
- **THEN** it MUST render no glyph, no tinted border, and no progress bar

#### Scenario: A parent that is also filtered out of view

- **WHEN** an epic's children are hidden by an active filter but the parent is visible
- **THEN** the counter MUST still reflect the full board, not the filtered subset

### Requirement: Terminal columns are marked in the list header

A column whose name appears in the board's `done_columns` SHALL render a check mark in its list header, between the column title and the card count, reusing the existing `IconCheck` component.

The marker MUST use a muted foreground token so it reads as metadata rather than as an action. It MUST carry an accessible label identifying the column as terminal, because a check mark alone does not say what it marks.

The `*` marker from `kanban.toml` MUST NEVER be rendered.

#### Scenario: A terminal column shows the marker

- **WHEN** `done_columns` contains `"done"`
- **THEN** the `done` list header renders a check mark between the title and the count

#### Scenario: A non-terminal column shows nothing extra

- **WHEN** `done_columns` does not contain `"todo"`
- **THEN** the `todo` list header renders no check mark

#### Scenario: Multiple terminal columns are each marked

- **WHEN** `done_columns` equals `["shipped","wont-fix"]`
- **THEN** both list headers render the marker

#### Scenario: The marker is labelled

- **WHEN** a terminal column's marker is rendered
- **THEN** it MUST expose an accessible label describing the column as terminal

### Requirement: The detail modal reports the epic relation read-only

The card-detail modal SHALL report a card's epic relation without offering any way to change it in this change.

For a card carrying an `epic`, the modal MUST render a labelled row naming the parent, showing the parent's colored chip and its id. For a card referenced as an epic, the modal MUST instead render its children — each with its title and column — plus the progress bar and `done/total` counter. A card MUST NEVER show both sections, because one-level nesting makes that state unrepresentable.

A card that neither carries nor is referenced as an epic MUST render no relation section at all, leaving the modal identical to its current layout.

The sections MUST NOT expose an add, remove, reassign, or color-picking control. Every element in them is presentational.

#### Scenario: A child shows its parent

- **WHEN** the modal opens on a card carrying `epic: "rl4m9x"`
- **THEN** it renders a section naming card `rl4m9x`, with that card's chip and id
- **AND** it renders no children section

#### Scenario: A parent shows its children and progress

- **WHEN** the modal opens on a card referenced by three others, one of which is in a terminal column
- **THEN** it renders a list of those three cards with their titles and columns
- **AND** it renders a progress bar and the counter `1/3`
- **AND** it renders no parent section

#### Scenario: Children are listed in board order

- **WHEN** an epic's children appear in the `/api/board` `cards` array in the order `[c, a, b]`
- **THEN** the modal lists them in that order

#### Scenario: An unrelated card shows no section

- **WHEN** the modal opens on a card that carries no `epic` and is referenced by none
- **THEN** the modal MUST render neither a parent nor a children section

#### Scenario: No editing affordance is present

- **WHEN** the modal renders either relation section
- **THEN** it MUST NOT render a control to add, remove, reassign, or recolor the relation

## MODIFIED Requirements

### Requirement: Wire shape ↔ UI shape adapter

`app.jsx` SHALL contain a single adapter boundary that converts the
server's `/api/board` JSON envelope (`{ columns[], done_columns[],
cards[{ id, title, column, priority, tags, description, epic, color,
created_at, updated_at }], priorities[], priority_colors{},
project_name }`) into the React component tree's working shape (`{
title, lists: [{ id, title, done, cards: [{ id, text, tags, priority,
description, epic, color, createdAt, updatedAt }] }] }`) and vice versa
for outbound mutations. List identity MUST be the server column name
(not a synthetic UUID). Mutation handlers MUST translate UI-shape values
to server-shape request bodies before calling fetch.

Because the wire carries no denormalized relation data, the adapter is
the single place that resolves relations. It MUST build an id → card
index over the full `cards` array once per board load, and expose from
it: a card's parent card, a card's children in payload order, and a
card's `done`/`total` counts. Components MUST read those derived values
rather than scanning the board themselves, so the resolution cost is
paid once per load rather than once per rendered card.

A list's `done` flag MUST be derived from membership in `done_columns`.

#### Scenario: Server load translates to UI shape

- **WHEN** the server returns a card `{ id:"X", title:"Hi",
  column:"todo", created_at:"2026-01-01T00:00:00Z" }`
- **THEN** the React tree exposes that card as `{ id:"X",
  text:"Hi", createdAt:"2026-01-01T00:00:00Z" }` inside
  `lists.find(l => l.id === "todo").cards`

#### Scenario: List id is the column name

- **WHEN** the server has columns `["backlog","todo","done"]`
- **THEN** the React tree's `lists` array has `id` values
  `"backlog"`, `"todo"`, `"done"` in order

#### Scenario: Epic and color survive the adapter

- **WHEN** the server returns a card with `epic: "rl4m9x"` and another
  with `color: "#8b5cf6"`
- **THEN** the UI-shape cards expose `epic` and `color` with the same
  values

#### Scenario: The adapter resolves a parent by id

- **WHEN** the payload contains card `f20wbo` with `epic: "rl4m9x"` and
  card `rl4m9x` titled `"Card relations"`
- **THEN** the adapter exposes `"Card relations"` as `f20wbo`'s parent
  title without any component re-scanning the board

#### Scenario: The adapter derives children and counts

- **WHEN** three cards carry `epic: "rl4m9x"` and one of them sits in a
  column listed in `done_columns`
- **THEN** the adapter exposes `rl4m9x`'s children in payload order
- **AND** exposes its counts as `done = 1`, `total = 3`

#### Scenario: List done flag comes from done_columns

- **WHEN** the server returns `columns: ["todo","done"]` and
  `done_columns: ["done"]`
- **THEN** the UI-shape list `done` has `done === true`
- **AND** the list `todo` has `done === false`

#### Scenario: A pre-epic payload adapts unchanged

- **WHEN** the server returns a payload in which no card carries `epic`
  or `color` and `done_columns` is `[]`
- **THEN** the resulting UI shape MUST be equivalent to the pre-epic
  adapter output
