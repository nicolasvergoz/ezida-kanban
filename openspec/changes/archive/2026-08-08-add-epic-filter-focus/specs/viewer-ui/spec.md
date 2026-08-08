# Viewer UI Specification (delta)

## ADDED Requirements

### Requirement: The adapter exposes the board's epics in payload order

The wire↔UI adapter's epic index SHALL expose, in addition to the
per-card lookups it already provides, the list of cards that are
referenced as an epic by at least one other card, in `/api/board`
`cards` array order.

The list MUST be derived from the full payload once per board load,
so it is unaffected by an active filter — a filter that hides every
child of an epic MUST NOT remove that epic from the list, or focusing
it would clear the very control the user just used.

A card carrying a `color` but referenced by nobody MUST NOT appear in
the list.

#### Scenario: Epics are listed in payload order

- **WHEN** the `cards` array contains, in order, cards `a`, `p2`,
  `b`, `p1`, and cards `a` and `b` carry `epic: "p1"` while `p2` is
  referenced by no card
- **THEN** the exposed list MUST contain exactly `p1`

#### Scenario: Two epics keep payload order

- **WHEN** the `cards` array lists parent `p2` before parent `p1`,
  and both are referenced by at least one card
- **THEN** the exposed list MUST be `[p2, p1]`

#### Scenario: A board with no epic relation exposes an empty list

- **WHEN** no card on the board carries an `epic`
- **THEN** the exposed list MUST be empty

#### Scenario: The list ignores the active filter

- **WHEN** an epic's every child is hidden by an active filter
- **THEN** that epic MUST still appear in the exposed list

### Requirement: The filter popover exposes an epic scope

The filter popover SHALL render an `Epic` section below the
`Priority` section, containing one pill per epic on the board plus a
trailing `No epic` pill. Each epic pill MUST carry a color dot filled
from that epic's `color` and MUST be labelled with the parent card's
title.

Each pill MUST toggle membership of its epic's id in the filter's
epic set, and MUST render an `aria-pressed` attribute reflecting that
membership. The set MUST default to empty, meaning every card passes
the dimension.

Pills MUST be listed in the order the adapter exposes the board's
epics. A pill label MUST truncate with an ellipsis at the pill's
maximum width and MUST carry the untruncated title in a `title`
attribute.

When the board has no epic relation, the popover MUST NOT render the
`Epic` section at all — neither the heading, the pills, nor the
`No epic` pill.

#### Scenario: One pill per epic, with the epic's color

- **WHEN** the board contains card `rl4m9x` titled `Card relations`
  with `color: "#8b5cf6"`, referenced by two other cards, and the
  user opens the filter popover
- **THEN** the popover MUST render an `Epic` section containing a
  pill labelled `Card relations`
- **AND** that pill's dot MUST be filled from `#8b5cf6`

#### Scenario: Clicking a pill toggles it

- **WHEN** an epic pill is off and the user clicks it
- **THEN** its `aria-pressed` MUST become `true`
- **AND** clicking it again MUST return `aria-pressed` to `false`

#### Scenario: Two epics can be selected at once

- **WHEN** the user clicks the pills of two different epics
- **THEN** both pills MUST report `aria-pressed` as `true`
- **AND** cards belonging to either epic MUST remain visible

#### Scenario: Pills follow the adapter's epic order

- **WHEN** the adapter exposes epics in the order `[p2, p1]`
- **THEN** the popover MUST render `p2`'s pill before `p1`'s

#### Scenario: A long epic title truncates

- **WHEN** an epic's title exceeds the pill's maximum width
- **THEN** the visible label MUST be truncated with an ellipsis
- **AND** the pill's `title` attribute MUST hold the full title

#### Scenario: A board with no epics renders no section

- **WHEN** no card on the board carries an `epic` and the user opens
  the filter popover
- **THEN** the popover MUST NOT render an `Epic` heading
- **AND** the popover MUST be identical to its pre-epic rendering

#### Scenario: Clear all resets the epic scope

- **WHEN** at least one epic pill is on and the user clicks
  `Clear all`
- **THEN** every epic pill MUST report `aria-pressed` as `false`
- **AND** every card on the board MUST be rendered as visible

### Requirement: A focused epic keeps its parent card visible

When the filter's epic set contains an id, a card SHALL pass the epic
dimension when its `epic` equals that id **or** when its own id
equals that id.

The second clause is normative, not incidental: an epic's parent card
carries no `epic` field, so matching on `epic` alone would hide the
parent — and with it the glyph, the tinted border, the progress bar,
and the `done/total` counter that are the reason to focus an epic.

#### Scenario: The parent survives its own focus

- **WHEN** cards `a`, `b`, `c` carry `epic: "p1"` and the user
  activates the pill for `p1`
- **THEN** cards `a`, `b`, `c` MUST remain visible
- **AND** card `p1` MUST remain visible
- **AND** every other card on the board MUST be hidden

#### Scenario: The parent's progress still reads the whole board

- **WHEN** an epic with three children, one in a terminal column, is
  focused
- **THEN** the parent's counter MUST read `1/3`

#### Scenario: Focusing one epic hides another epic's cards

- **WHEN** the board carries two epics and the user focuses only the
  first
- **THEN** the second epic's parent and children MUST all be hidden

### Requirement: The `No epic` scope matches cards unrelated to any epic

The `No epic` pill SHALL match a card that neither carries an `epic`
nor is referenced as one by another card.

A parent card MUST NOT match `No epic`. A card that six others point
at is the epic, not a card without one; matching on the absence of
the `epic` field alone would list every parent under `No epic`.

The pill MUST participate in the same epic set as the named pills, so
selecting it alongside a named epic MUST show the union of both.

#### Scenario: An unrelated card matches

- **WHEN** card `x` carries no `epic` and is referenced by no card,
  and the user activates `No epic`
- **THEN** card `x` MUST remain visible

#### Scenario: A parent card does not match

- **WHEN** card `p1` is referenced by three cards and the user
  activates `No epic`
- **THEN** card `p1` MUST be hidden

#### Scenario: A child card does not match

- **WHEN** card `a` carries `epic: "p1"` and the user activates
  `No epic`
- **THEN** card `a` MUST be hidden

#### Scenario: Combined with a named epic, the scopes union

- **WHEN** the user activates both `No epic` and the pill for `p1`
- **THEN** `p1`, its children, and every card unrelated to any epic
  MUST all remain visible

### Requirement: A card's epic chip activates that epic's scope

The epic chip rendered in a card's `.card-foot` SHALL be interactive.
Clicking it MUST toggle its epic's id in the filter's epic set,
matching the tag chip, which already edits the filter on click.

Clicking the chip MUST NOT also open the card's detail modal — the
chip MUST be excluded from the card-click handler the same way
`.card-tag-chip`, `.card-tag-add`, `.card-tag-input`, and
`.card-delete` already are.

The chip rendered inside the card-detail modal's parent row MUST
remain inert: it sits above a board the user cannot see, so setting a
scope from it would have no observable effect until the modal closes.

#### Scenario: Clicking a chip focuses its epic

- **WHEN** card `a` carries `epic: "p1"` and the user clicks its
  epic chip
- **THEN** the filter's epic set MUST contain `p1`
- **AND** the popover's `p1` pill MUST report `aria-pressed` as
  `true`
- **AND** only `p1` and its children MUST remain visible

#### Scenario: Clicking a chip does not open the modal

- **WHEN** the user clicks a card's epic chip
- **THEN** no card-detail modal MUST be rendered

#### Scenario: Clicking the chip of a focused epic unfocuses it

- **WHEN** epic `p1` is focused and the user clicks the epic chip of
  one of its children
- **THEN** the filter's epic set MUST NOT contain `p1`
- **AND** every card on the board MUST be rendered as visible

#### Scenario: The modal's parent chip is inert

- **WHEN** the card-detail modal is open on a card carrying an
  `epic` and the user clicks the chip in its parent row
- **THEN** the filter's epic set MUST be unchanged
- **AND** the modal MUST remain open

## MODIFIED Requirements

### Requirement: Filter matches title, description, tags, and card ID case-insensitively

The filter SHALL perform a case-insensitive substring match against
each card's title, description, tag values, and ID. The popover SHALL
expose four independent scope toggles — Title, Description, Tags, and
ID — that gate which fields the query is tested against. Every
keystroke in the filter input MUST update the rendered set of visible
cards. Whitespace-only queries MUST be treated as an empty filter
(every card visible). When all four scope toggles are off and the
query is non-empty, no card MUST match.

The query, the priority set, and the epic set are independent
dimensions combined with AND: a card MUST satisfy every dimension
that is set to remain visible. Within a dimension, membership is OR —
two selected priorities, or two selected epics, widen that dimension
rather than narrowing it. A dimension left at its default (empty
query, empty set) MUST let every card pass.

#### Scenario: Title substring match

- **WHEN** the board contains a card with title `Refactor auth flow`
  and the user types `auth` into the filter input
- **THEN** that card MUST remain visible
- **AND** cards whose title, description, tags, and id contain no
  `auth` substring MUST be hidden

#### Scenario: Case-insensitive match

- **WHEN** the board contains a card with title `Refactor AUTH flow`
  and the user types `auth` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: Description substring match

- **WHEN** the board contains a card with title `Cleanup` and
  description `replace the legacy auth call with the new one` and
  the user types `auth` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: Tag substring match

- **WHEN** the board contains a card with tags `["security",
  "tech-debt"]` and the user types `secur` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: ID substring match

- **WHEN** the board contains a card with id `f20wbo` and the user
  types `f20wbo` into the filter input
- **THEN** that card MUST remain visible
- **AND** cards whose id, title, description, and tags contain no
  `f20wbo` substring MUST be hidden

#### Scenario: ID partial substring match

- **WHEN** the board contains a card with id `f20wbo` and the user
  types `f20` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: ID match is case-insensitive

- **WHEN** the board contains a card with id `F20WBO` and the user
  types `f20` into the filter input
- **THEN** that card MUST remain visible

#### Scenario: ID scope can be turned off

- **WHEN** the user types a card's id into the filter input
- **AND** the ID scope pill in the popover is toggled off
- **AND** the Title, Description, and Tags scope pills are also off,
  OR none of those fields contain the id substring
- **THEN** that card MUST NOT be matched by the filter

#### Scenario: Empty filter shows everything

- **WHEN** the filter input is empty
- **THEN** every card on the board MUST be rendered as visible
- **AND** no `No matches` placeholder MUST be rendered

#### Scenario: Whitespace-only filter shows everything

- **WHEN** the filter input contains only spaces
- **THEN** every card on the board MUST be rendered as visible

#### Scenario: Epic scope and query combine with AND

- **WHEN** epic `p1` is focused and the user types `auth` into the
  filter input
- **THEN** only cards belonging to `p1` — or `p1` itself — whose
  searched fields contain `auth` MUST remain visible

#### Scenario: Epic scope and priority scope combine with AND

- **WHEN** epic `p1` is focused and the `high` priority pill is on
- **THEN** only `high` cards belonging to `p1`, or `p1` itself when
  it is `high`, MUST remain visible

#### Scenario: An empty epic set lets every card pass

- **WHEN** no epic pill is selected
- **THEN** the epic dimension MUST NOT hide any card

### Requirement: Filter button shows active state and mono-counter badge when filter is non-empty

When any filter dimension is set — a non-empty query, a non-empty
priority set, or a non-empty epic set — the Filter button SHALL
render in its active state (surface fill) and SHALL display a
mono-counter badge whose text content is the total count of matching
cards across the entire board. When every dimension is at its
default, the active state and the badge MUST NOT be rendered.

#### Scenario: Active state appears when filter is non-empty

- **WHEN** the user types any non-empty value into the filter input
- **THEN** the Filter button element MUST carry a CSS class
  indicating active state (e.g. `state-active`)

#### Scenario: Active state appears when only an epic is focused

- **WHEN** the filter input is empty and the user activates one epic
  pill
- **THEN** the Filter button MUST carry the active-state class
- **AND** the badge MUST be rendered

#### Scenario: Mono-counter badge shows total board-wide match count

- **WHEN** the board contains 12 cards total across all columns,
  and 4 of them match the current filter text
- **THEN** the Filter button MUST render a badge element with
  mono-counter typography
- **AND** the badge's text content MUST be `4`

#### Scenario: A focused epic counts its parent

- **WHEN** an epic with three children is the only filter set
- **THEN** the badge's text content MUST be `4`

#### Scenario: Match count updates on every keystroke

- **WHEN** the user types one additional character into the filter
  input such that the number of matching cards changes from 4 to 1
- **THEN** the Filter button badge's text content MUST update to
  `1`

#### Scenario: Clearing the filter removes the active state and badge

- **WHEN** the filter input is non-empty and the user clears it
  (either by editing the input to empty or by clicking the
  `Clear filter` inline link)
- **THEN** the Filter button MUST NOT carry the active-state class
- **AND** the badge element MUST NOT be rendered (or MUST be hidden
  such that its text content is not visible)

#### Scenario: Clear filter link is visible only when filter is non-empty

- **WHEN** the filter input is empty
- **THEN** the popover MUST NOT render a visible `Clear filter`
  link

- **WHEN** the filter input is non-empty
- **THEN** the popover MUST render a visible `Clear filter` link
  below the input

#### Scenario: Clear filter link empties the filter

- **WHEN** the filter input contains `auth` and the user clicks the
  `Clear filter` link
- **THEN** the filter input MUST become empty
- **AND** every card on the board MUST be rendered as visible
- **AND** the popover MUST remain open

### Requirement: Filter state is transient and not persisted

The filter text, the scope toggles, the priority set, the epic set,
and the popover open state SHALL exist only in the component state.
The page MUST NOT write any of them to `localStorage`,
`sessionStorage`, cookies, or the URL. A page reload MUST reset every
filter dimension and the popover open state to their defaults.

#### Scenario: Reload clears the filter

- **WHEN** the user has typed `auth` into the filter input and then
  reloads the page
- **THEN** the filter input MUST be empty after reload
- **AND** every card on the board MUST be rendered as visible

#### Scenario: Reload clears a focused epic

- **WHEN** the user has focused an epic and then reloads the page
- **THEN** no epic pill MUST report `aria-pressed` as `true`
- **AND** every card on the board MUST be rendered as visible

#### Scenario: No localStorage write

- **WHEN** the user types into the filter input
- **THEN** no `localStorage` entry related to the filter (e.g. a key
  matching `*filter*` or `*query*`) MUST be created
