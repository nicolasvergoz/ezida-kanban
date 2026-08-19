## ADDED Requirements

### Requirement: A virtual Archive column renders at the end of the board, collapsed by default

When `GET /api/board`'s response carries a non-empty `archived_cards`
array, the page SHALL render one additional column-like section after
every real column, marked `data-archive="true"` and never
`data-column="archive"` — a real user column literally named `archive`
remains addressable by existing selectors, since the two are distinct
DOM markers. The Archive section is NOT an entry of the board's column
list: it does not participate in column drag-reorder, column rename, or
column delete.

The Archive section SHALL render **collapsed** by default: a narrow
strip showing an icon and the archived-card count. Clicking it expands
it in place to a normal-width column listing the archived cards;
clicking again collapses it. The expand/collapse state is local to the
page session and is NOT persisted.

When `archived_cards` is absent from the response (empty or missing
archive), the page SHALL render no Archive section at all — no
collapsed strip, no `data-archive` element anywhere in the DOM. A board
that has never archived anything MUST therefore be pixel-identical to
one rendered before this capability existed.

#### Scenario: Archive section appears when cards are archived

- **WHEN** the board response includes a non-empty `archived_cards`
  array
- **THEN** the page contains exactly one element matching
  `[data-archive="true"]`
- **AND** it appears after every element matching `.list[data-column]`

#### Scenario: Archive section is absent for a board with nothing archived

- **WHEN** the board response has no `archived_cards` key
- **THEN** the page contains zero elements matching `[data-archive]`

#### Scenario: Collapsed by default, shows the count

- **WHEN** the Archive section first renders with 5 archived cards
- **THEN** it is in its collapsed (narrow strip) state
- **AND** the strip displays the number `5`

#### Scenario: Clicking the collapsed strip expands it

- **WHEN** the user clicks the collapsed Archive strip
- **THEN** the section expands to show the archived cards
- **AND** clicking it again collapses it back to the strip

#### Scenario: A real column named "archive" is unaffected

- **WHEN** the board has a real column whose name is `archive`
- **THEN** `[data-column="archive"]` matches that real column, not the
  virtual Archive section
- **AND** the virtual Archive section (if rendered) matches
  `[data-archive="true"]` and nothing else

### Requirement: Archived cards render read-only with their archive metadata

Within the expanded Archive section, each archived card SHALL render
using the same visual chrome as a live card (id, title, priority pill,
tags) plus two archive-specific pieces of information: the date it was
archived and the column it was archived from. It MUST NOT expose the
affordances a live card exposes for direct editing: no delete corner
button, no tag add/remove, no click-to-open the live edit modal, and no
epic-chip filter click-through.

Archived cards MUST NOT be counted by the topbar's active-filter count,
and MUST NOT be included when computing any live epic's `done`/`total`
progress — the epic index that progress is computed from is built from
live cards only, so an archived parent does not resurrect as an epic
and an archived child does not inflate a live parent's denominator.

#### Scenario: Archived card shows its archive date and origin column

- **WHEN** an archived card whose stored column is `done` and whose
  `archived_at` is `2026-08-19T10:00:00Z` renders
- **THEN** the card displays a short date derived from `archived_at`
- **AND** the card displays `done` as its origin column

#### Scenario: Archived cards are read-only

- **WHEN** an archived card renders
- **THEN** it exposes no delete button, no tag-add control, and no
  click target that opens the live editable detail modal

#### Scenario: Archived cards do not affect live epic progress

- **WHEN** an epic's child is archived
- **THEN** the epic's `done`/`total` progress, as rendered on the live
  board, does not count that child

### Requirement: Clicking an archived card opens a read-only detail view with a Restore action

Clicking an archived card SHALL open a detail view reusing the same
modal chrome as the live detail modal, with every field rendered
read-only, plus a single primary action: Restore. Activating Restore
SHALL call the unarchive endpoint for that card and close the view on
success; on failure the view SHALL surface the error and remain open.

#### Scenario: Restoring an archived card

- **WHEN** the user opens an archived card's detail view and activates
  Restore
- **THEN** an unarchive request is sent for that card's id
- **AND** on success, the card no longer appears in the Archive section
- **AND** the card now appears in its restored column

### Requirement: The card detail modal exposes an Archive action

The live card detail modal's action row SHALL gain an Archive button
alongside the existing delete action. Activating it on a card that is
an epic with children SHALL first ask for confirmation (mirroring the
existing delete confirmation), naming how many additional cards will be
archived. Activating it on any other card archives immediately with no
confirmation. On success the modal closes and the card leaves its
column.

#### Scenario: Archiving a standalone card needs no confirmation

- **WHEN** the user opens a card with no epic children and activates
  Archive
- **THEN** the archive request is sent immediately with no confirmation
  prompt

#### Scenario: Archiving an epic asks for confirmation first

- **WHEN** the user opens a card that is an epic with two children and
  activates Archive
- **THEN** a confirmation naming the 2 additional cards appears before
  any request is sent

#### Scenario: Successful archive closes the modal

- **WHEN** an archive action succeeds
- **THEN** the detail modal closes
- **AND** the archived card no longer appears among the live column's
  cards

### Requirement: The column menu exposes an "Archive all cards" action

Each column's `⋯` menu (`ListMenu`) SHALL gain a third entry, "Archive
all cards", positioned between the existing "Terminal column" toggle
and "Delete list". The entry MUST be hidden entirely when the column
has no cards. Activating it on a column whose cascade would also
archive cards from other columns SHALL first ask for confirmation,
naming how many additional cards are affected; activating it on a
column with no such cascade archives immediately.

#### Scenario: Entry is hidden on an empty column

- **WHEN** the ⋯ menu opens for a column with zero cards
- **THEN** the menu does not contain an "Archive all cards" entry

#### Scenario: Archiving a column with no outside cascade needs no confirmation

- **WHEN** "Archive all cards" is activated for a column whose cards
  have no epic relationships reaching outside the column
- **THEN** the archive request is sent immediately

#### Scenario: Archiving a column with an outside cascade asks first

- **WHEN** "Archive all cards" is activated for a column containing an
  epic whose child lives in a different column
- **THEN** a confirmation naming the affected outside cards appears
  before any request is sent

#### Scenario: Archiving a column unblocks deleting it

- **WHEN** "Archive all cards" succeeds on a column that previously
  had cards, and "Delete list" is then activated on the same column
- **THEN** the column is removed
