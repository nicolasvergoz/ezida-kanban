# demo-viewer Specification

## Purpose

Provide a static, in-memory demo of the Ezida viewer at `/demo/` on the public landing page, so prospects can experience the UX without installing the binary. The demo loads Ezida's own board snapshot as fixture content; all mutations are ephemeral.
## Requirements
### Requirement: Demo viewer hosted at `/demo/`
The landing page deployment SHALL include a static demo of the viewer at the `/demo/` path. The demo SHALL load Ezida's own board as fixture content and present the same UI as `ezida serve`.

#### Scenario: Visitor opens the demo URL
- **WHEN** a visitor navigates to `<base-url>/demo/`
- **THEN** the page loads and renders the same Kanban UI as `ezida serve`, populated with the cards from the deployed `board.json` snapshot

#### Scenario: Demo on a board-only commit
- **WHEN** the only change in a commit on `main` is to `kanban.toml`
- **THEN** the Pages workflow runs and the redeployed demo reflects the new board state

### Requirement: Demo mutations are in-memory only
All viewer mutations (drag a card, edit a title, rename a column, delete a card, add a column, reorder columns) SHALL be applied to an in-memory copy of the board state inside the browser and SHALL NOT be persisted across page reloads.

#### Scenario: Drag a card then refresh
- **WHEN** a visitor drags a card to a different column and then reloads the page
- **THEN** the card is back in its original column from `board.json`

#### Scenario: Mutation succeeds visually
- **WHEN** a visitor drags a card from column A to column B
- **THEN** the card immediately appears in column B in the UI and remains there until reload

### Requirement: SSE is stubbed out in the demo
The demo viewer SHALL NOT open any real `EventSource` connection. The `connected` status indicator MAY still report "connected" so the UI does not show a disconnect warning.

#### Scenario: No network connection to /api/events
- **WHEN** the demo loads
- **THEN** no HTTP request to `/api/events` is made

### Requirement: Demo banner identifies the snapshot
The demo page SHALL display a visible banner reading `Demo — snapshot <sha7> · changes don't persist`, where `<sha7>` is the first 7 characters of the commit that produced the deploy.

#### Scenario: Deployed demo shows SHA
- **WHEN** the demo is loaded from Pages
- **THEN** the banner contains the 7-char prefix of `$GITHUB_SHA` of the producing commit, not the literal placeholder text

#### Scenario: Local preview before CI substitution
- **WHEN** the demo is opened from a local checkout that has not gone through the CI substitution step
- **THEN** the banner contains the sentinel `dev` instead of an unresolved placeholder

### Requirement: Demo shares viewer assets via symlinks

The demo directory SHALL link viewer assets via symlinks so the
demo stays byte-identical to the real viewer: `site/demo/app.jsx`,
`site/demo/styles.css`, and `site/demo/vendor` MUST be symbolic
links into `internal/server/web/`. The legacy symlinks
`site/demo/app.js` and `site/demo/style.css` MUST NOT exist (the
files they pointed at no longer exist either).

#### Scenario: Asset divergence audit

- **WHEN** `site/demo/app.jsx` is inspected
- **THEN** it is a symlink whose target resolves to
  `internal/server/web/app.jsx`

- **WHEN** `site/demo/styles.css` is inspected
- **THEN** it is a symlink whose target resolves to
  `internal/server/web/styles.css`

- **WHEN** `site/demo/vendor` is inspected
- **THEN** it is a symlink whose target resolves to
  `internal/server/web/vendor`

