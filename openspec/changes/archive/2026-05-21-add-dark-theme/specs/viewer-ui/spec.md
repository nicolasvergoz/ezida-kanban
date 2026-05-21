## ADDED Requirements

### Requirement: Topbar exposes a 3-state theme toggle

The rendered topbar SHALL include a theme toggle composed of exactly
three buttons representing the user's choice: Light, System, and Dark.
Each button MUST carry a stable `data-theme-choice` attribute whose
value is `"light"`, `"system"`, or `"dark"` respectively, and an icon
(sun, monitor, moon). The currently-selected button MUST carry
`aria-pressed="true"`; the other two MUST carry `aria-pressed="false"`.
The default selected button SHALL be the one whose `data-theme-choice`
equals `"system"` when no prior preference is stored.

#### Scenario: Three toggle buttons present in the topbar

- **WHEN** the page loads with no `ezida.theme` value in `localStorage`
- **THEN** the topbar contains exactly three elements matching
  `[data-theme-choice]`
- **AND** their `data-theme-choice` values in DOM order are `"light"`,
  `"system"`, `"dark"`
- **AND** the button with `data-theme-choice="system"` has
  `aria-pressed="true"`
- **AND** the other two buttons have `aria-pressed="false"`

#### Scenario: Clicking a segment changes the active state

- **WHEN** the user clicks the `data-theme-choice="dark"` button
- **THEN** that button MUST carry `aria-pressed="true"`
- **AND** the two other buttons MUST carry `aria-pressed="false"`

#### Scenario: System default derives from OS preference

- **WHEN** the page loads with no stored preference AND the browser's
  `prefers-color-scheme` is `dark`
- **THEN** `document.documentElement.getAttribute("data-theme")` MUST
  equal `"dark"`
- **AND** the System segment MUST be the one with `aria-pressed="true"`

#### Scenario: System mode reacts live to OS theme change

- **WHEN** the toggle is set to System AND the OS-level
  `prefers-color-scheme` flips from `light` to `dark` during the
  session (e.g. macOS automatic dusk transition)
- **THEN** `document.documentElement.getAttribute("data-theme")` MUST
  update to `"dark"` without a page reload
- **AND** the System segment MUST remain the one with
  `aria-pressed="true"` (the user's choice did not change)

### Requirement: Dark color tokens override light values via `[data-theme="dark"]`

The stylesheet at `/static/style.css` SHALL contain a rule block whose
selector is `[data-theme="dark"]` and which reassigns the same CSS
custom properties defined under `:root` (at minimum `--bg-base`,
`--surface`, `--surface-2`, `--border`, `--border-strong`, `--text`,
`--text-muted`, `--text-faint`) to dark-theme values per
`refs/design.md` §"Colors". No component selector outside of
`:root` / `[data-theme="dark"]` MAY embed a hex literal that depends
on a specific theme.

#### Scenario: Stylesheet exposes the dark selector

- **WHEN** `GET /static/style.css` is fetched from the running server
- **THEN** the response body contains the literal substring
  `[data-theme="dark"]`

#### Scenario: Body background differs between light and dark

- **WHEN** the page is rendered with `data-theme="light"` (or no
  attribute) AND the computed `background-color` of `<body>` is read
- **THEN** the computed value reflects the light `--bg-base` (warm
  off-white `#fbfaf8` per design.md)
- **AND** when the same page is then switched to `data-theme="dark"`,
  the computed `background-color` of `<body>` MUST resolve to a value
  derived from the dark `--bg-base` (`#25282e` per design.md), so the
  two computed values MUST differ

### Requirement: Theme preference persists across reloads

The user's explicit choice from the toggle SHALL be written to
`localStorage` under the key `"ezida.theme"` with one of the literal
string values `"light"`, `"system"`, or `"dark"`. On subsequent page
loads, the stored value MUST drive the initial toggle state and the
initial `data-theme` attribute. The page MUST NOT throw if
`localStorage` is unavailable (e.g. private browsing, blocked by
policy) — the in-memory choice still drives the UI for the current
session but is not persisted.

#### Scenario: Choosing Dark persists across reload

- **WHEN** the user clicks the `data-theme-choice="dark"` button
- **THEN** `localStorage.getItem("ezida.theme")` MUST equal `"dark"`
- **AND** after a full page reload, the Dark segment MUST be the
  active one
- **AND** `document.documentElement.getAttribute("data-theme")` MUST
  equal `"dark"` on the reloaded page

#### Scenario: Choosing System persists across reload

- **WHEN** the user clicks the `data-theme-choice="system"` button
- **THEN** `localStorage.getItem("ezida.theme")` MUST equal `"system"`
- **AND** after reload, the System segment MUST be the active one
- **AND** the effective `data-theme` on `<html>` MUST resolve from
  the current `prefers-color-scheme`

#### Scenario: Invalid stored value falls back to System

- **WHEN** the page loads with `localStorage["ezida.theme"]` set to a
  value other than `"light"`, `"system"`, or `"dark"` (e.g. `"foo"`
  or stale corrupted state)
- **THEN** the System segment MUST be the active one
- **AND** the effective `data-theme` MUST be derived from
  `prefers-color-scheme`

#### Scenario: localStorage is blocked

- **WHEN** the page loads in an environment where `localStorage` reads
  and writes throw (private browsing, blocked storage)
- **THEN** the page MUST render without an uncaught exception in the
  console
- **AND** the System segment MUST be active by default
- **AND** clicking any toggle segment MUST update the active state
  and the `data-theme` attribute for the session, even though no
  value is persisted
