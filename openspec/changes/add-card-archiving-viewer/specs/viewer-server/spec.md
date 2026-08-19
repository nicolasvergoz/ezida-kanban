## MODIFIED Requirements

### Requirement: `GET /api/board` returns the current board

`GET /api/board` SHALL load `kanban.toml` from the current working
directory at request time and respond with a JSON object containing
`schema_version`, `columns`, `done_columns`, `priorities`,
`priority_colors`, `cards_per_column`, `cards`, and `project_name`. The
`cards` array MUST include the full `description` field for every card.
Response `Content-Type` MUST be `application/json`.

Each card object MAY carry `epic` (the six-character id of another card
on the board) and `color` (a CSS hex string). Both fields use
`omitempty` semantics — a card with neither produces a payload
byte-identical to the pre-epic shape.

The response MUST NOT carry any denormalized relation data: no
`epic_title`, no `epic_color`, no `children`, no `progress`. The
envelope already contains every card on the board, so a client resolves
a parent by id and counts children itself. Duplicating that data would
create a second source of truth that every mutation endpoint would have
to keep correct.

The top-level `columns` array MUST contain bare column names. Terminal
status is reported separately in `done_columns`, a string array listing
the names of columns whose cards count as done. The `*` marker used in
`kanban.toml` MUST NEVER appear in any response field. `done_columns`
MUST always be present, even when empty (`[]`), and every entry MUST
also appear in `columns`.

The top-level `project_name` field is a string set at server start
to `filepath.Base(filepath.Dir(<resolved boardPath>))` — i.e. the
parent-directory name of the resolved `kanban.toml` path. It MUST
fall back to the literal string `"Ezida"` when the computed
basename is empty, equal to `"."`, or equal to the platform path
separator. The value MUST NOT change for the lifetime of the
process (it is not re-evaluated when the board file changes).

The top-level `priority_colors` field is a JSON object mapping
priority name → hex color string. The server SHALL resolve it on
each request as follows:

1. Start with the user-provided `[board].priority_colors` map (may
   be empty or absent).
2. For each entry in the conventional default palette
   `{"low": "#22c55e", "medium": "#f59e0b", "high": "#ef4444"}`,
   if the priority name is declared in `[board].priorities` AND the
   user did not supply a color for it, fill in the default.
3. Drop any entry whose key is not in `[board].priorities` (defense
   in depth; validation already rejects this at load time).

The field MUST always be present, even when empty (`{}`). User
values MUST always win over defaults.

The response SHALL additionally carry a top-level `archived_cards`
array with `omitempty` semantics: the server also loads the archive at
the sibling path derived from the board path, reconciling it against
the live board (a card present in both — the residue of a crash between
the two writes of an archive operation — is dropped from this array,
never from `cards`). Each element is the same per-card shape as `cards`
plus one additional key, `archived_at`. When the reconciled archive is
empty, `archived_cards` is entirely absent from the response — not an
empty array — so a board that has never archived anything produces a
response byte-identical to one from before this capability existed. A
missing `kanban.archive.toml` is treated identically to an empty one.

#### Scenario: Valid board

- **WHEN** `GET /api/board` is called against a server whose
  `kanban.toml` contains 2 columns and 3 cards
- **THEN** the response status is `200`
- **AND** `Content-Type` is `application/json`
- **AND** the body's `schema_version` equals `2`
- **AND** `cards_per_column` reflects the per-column count
- **AND** each card in `cards` has a `description` field (may be
  empty string)
- **AND** the body contains a top-level string field `project_name`
- **AND** the body contains a top-level object field `priority_colors`
  (possibly empty)
- **AND** the body contains a top-level array field `done_columns`
  (possibly empty)

#### Scenario: Project name reflects parent directory

- **WHEN** `GET /api/board` is called against a server whose
  resolved board path is `/tmp/my-project/kanban.toml`
- **THEN** the response body's `project_name` equals `"my-project"`

#### Scenario: Project name falls back to "Ezida" at filesystem root

- **WHEN** `GET /api/board` is called against a server whose
  resolved board path produces an empty or `"."` parent-directory
  basename
- **THEN** the response body's `project_name` equals `"Ezida"`

#### Scenario: Project name is stable across requests

- **WHEN** `GET /api/board` is called twice against the same
  running server with a board file rewritten in between
- **THEN** both responses contain the same `project_name` value

#### Scenario: Board file missing

- **WHEN** `GET /api/board` is called and no `kanban.toml` exists at
  the resolved path
- **THEN** the response status is `500`
- **AND** the body is `{"error":{"code":"BOARD_NOT_FOUND",...}}`

#### Scenario: Board file has wrong schema version

- **WHEN** `GET /api/board` is called against a `kanban.toml` whose
  `schema_version` is not `2`
- **THEN** the response status is `500`
- **AND** the body's `error.code` is `SCHEMA_VERSION_MISMATCH`

#### Scenario: Defaults fill in low/medium/high when declared

- **WHEN** `GET /api/board` is called against a board whose
  `[board].priorities = ["low", "medium", "high"]` and
  `[board.priority_colors]` is absent
- **THEN** the response body's `priority_colors` equals
  `{"low":"#22c55e","medium":"#f59e0b","high":"#ef4444"}`

#### Scenario: User values override defaults

- **WHEN** the user-supplied `[board.priority_colors]` contains
  `high = "#000000"` and priorities include `low`, `medium`, `high`
- **THEN** the response body's `priority_colors.high` equals
  `"#000000"`
- **AND** `priority_colors.low` and `priority_colors.medium` equal
  the default green and orange

#### Scenario: Custom priority names without defaults are absent

- **WHEN** `[board].priorities = ["urgent"]` and
  `[board.priority_colors]` is absent
- **THEN** the response body's `priority_colors` equals `{}`

#### Scenario: Custom priority names with explicit colors are returned

- **WHEN** `[board].priorities = ["urgent"]` and
  `[board.priority_colors] = { urgent = "#ff0000" }`
- **THEN** the response body's `priority_colors` equals
  `{"urgent":"#ff0000"}`

#### Scenario: Epic and color are carried per card

- **WHEN** `GET /api/board` is called against a board where card
  `f20wbo` has `epic = 'rl4m9x'` and card `rl4m9x` has
  `color = '#8b5cf6'`
- **THEN** the `f20wbo` object's `epic` equals `"rl4m9x"`
- **AND** the `rl4m9x` object's `color` equals `"#8b5cf6"`

#### Scenario: Cards without epic or color omit the keys

- **WHEN** `GET /api/board` is called against a board where no card
  carries an `epic` or a `color`
- **THEN** no object in `cards` contains an `epic` key
- **AND** no object in `cards` contains a `color` key

#### Scenario: No relation data is denormalized

- **WHEN** `GET /api/board` is called against a board containing an
  epic with three children
- **THEN** no object in `cards` contains `epic_title`, `epic_color`,
  `children`, or `progress`

#### Scenario: Terminal columns are reported separately from names

- **WHEN** `GET /api/board` is called against a board whose
  `[board].columns` is `['todo', 'shipped*', 'wont-fix*']`
- **THEN** the response body's `columns` equals
  `["todo","shipped","wont-fix"]`
- **AND** `done_columns` equals `["shipped","wont-fix"]`
- **AND** no string anywhere in the response contains a `*` character

#### Scenario: A board with no terminal column reports an empty array

- **WHEN** `GET /api/board` is called against a board where no column
  carries the marker
- **THEN** `done_columns` equals `[]`
- **AND** the response status is `200`

#### Scenario: A board with no archive omits archived_cards entirely

- **WHEN** `GET /api/board` is called against a board with no
  `kanban.archive.toml` on disk
- **THEN** the response body, decoded as a generic JSON object, MUST
  NOT contain an `archived_cards` key at all

#### Scenario: Archived cards carry archived_at alongside every live field

- **WHEN** `GET /api/board` is called against a board whose archive
  contains one card
- **THEN** `archived_cards` has length 1
- **AND** that entry carries every field `cards` entries carry, plus
  `archived_at`

#### Scenario: A duplicate left by a crashed archive operation is hidden

- **WHEN** `GET /api/board` is called against a board where the same
  id appears both in `kanban.toml` and in `kanban.archive.toml`
- **THEN** that id appears in `cards`
- **AND** that id does NOT appear in `archived_cards`

### Requirement: Server watches `kanban.toml` for external changes

The server SHALL start a `fsnotify`-based watcher covering both the
resolved board file and its sibling archive path at startup, and SHALL
re-arm each watch after Rename/Create/Remove events so atomic rewrites
(temp + rename) continue to be detected on either file. Because the
archive file usually does not exist at server start, the watcher MUST
NOT fail to start when the archive path is absent — an event on that
path fires from the moment it is first created onward. The watcher
MUST debounce a burst of events using a 200 ms timer before notifying
downstream consumers, coalescing events across both watched files into
one. The watcher MUST run for the lifetime of the server process and
MUST exit cleanly when the server's root context is cancelled. The
existing fail-fast contract is unchanged for the board file itself: the
server MUST still refuse to start when `kanban.toml` does not exist.

#### Scenario: Single external write fires one debounced event

- **WHEN** an external process atomically rewrites `kanban.toml` once
- **THEN** the watcher MUST deliver exactly one event on its `Events()` channel within 500 ms of the rewrite

#### Scenario: Burst of writes coalesces into one event

- **WHEN** an editor saves `kanban.toml` 3 times within 100 ms (simulating a fast typewriter / hot-reload tool)
- **THEN** the watcher MUST deliver at most 1 event within 500 ms following the burst

#### Scenario: Watcher survives a rename

- **WHEN** the file is replaced via temp + rename (the pattern used by `board.Save`) twice in a row, 1 s apart
- **THEN** both rewrites MUST produce a downstream event (re-arming the watch keeps it alive)

#### Scenario: Archive file created after server start still fires

- **WHEN** the server starts against a board with no `kanban.archive.toml`, and an external `ezida archive` then creates it
- **THEN** the watcher MUST deliver an event on its `Events()` channel within 500 ms of the archive file's creation

#### Scenario: Missing archive file at startup is not fatal

- **WHEN** the server starts against a board whose sibling
  `kanban.archive.toml` does not exist
- **THEN** the server MUST start successfully and bind its port

#### Scenario: An unrelated file in the same directory is ignored

- **WHEN** a file that is neither the board path nor the archive path
  is created or written in the board's directory
- **THEN** the watcher MUST NOT deliver an event on its `Events()`
  channel within 3× the debounce window

## ADDED Requirements

### Requirement: `POST /api/cards/{id}/archive` archives a card

`POST /api/cards/{id}/archive` SHALL archive the named card, cascading
to its epic children exactly as the CLI's `ezida archive <id>` does,
and respond `200` with `{"archived":"<id>","cascaded":[...]}` where
`cascaded` is `[]` rather than `null` when the operation did not
cascade. An unknown id MUST respond `404 CARD_NOT_FOUND` and leave both
files unchanged. A successful archive MUST cause the watcher to fire a
`board-changed` SSE event, per the existing "viewer's own writes are
tolerated" contract.

#### Scenario: Archiving a standalone card

- **WHEN** `POST /api/cards/a3f2k9/archive` is called for a card with
  no epic children
- **THEN** the response status is `200`
- **AND** the body is `{"archived":"a3f2k9","cascaded":[]}`
- **AND** a subsequent `GET /api/board` no longer lists `a3f2k9` in
  `cards`
- **AND** `a3f2k9` now appears in `archived_cards`

#### Scenario: Archiving an epic cascades its children

- **WHEN** `POST /api/cards/rl4m9x/archive` is called for an epic with
  three children
- **THEN** the response body's `cascaded` array has length 3

#### Scenario: Unknown id

- **WHEN** `POST /api/cards/zzzzzz/archive` is called and no such card
  exists
- **THEN** the response status is `404`
- **AND** the body's `error.code` is `CARD_NOT_FOUND`

### Requirement: `POST /api/cards/{id}/unarchive` restores a card

`POST /api/cards/{id}/unarchive` SHALL accept an optional JSON body
`{"column":"<name>"}` and restore the named archived card (and its
archived children) exactly as the CLI's `ezida unarchive <id>` does,
responding `200` with
`{"card":{...cardResponse},"cascaded":[...],"orphaned":[...],"relocated":<bool>}`.
An id not present in the archive MUST respond `404 CARD_NOT_ARCHIVED`.
An explicit `column` that does not exist on the board MUST respond
`400 COLUMN_NOT_FOUND`. An id already present on the live board MUST
respond `409 ID_COLLISION` and leave both files unchanged.

#### Scenario: Restoring into the stored column

- **WHEN** `POST /api/cards/a3f2k9/unarchive` is called with an empty
  body for a card whose stored column still exists
- **THEN** the response status is `200`
- **AND** the body's `card.column` equals the stored column
- **AND** the body's `relocated` is `false`

#### Scenario: Restoring with an explicit column

- **WHEN** `POST /api/cards/a3f2k9/unarchive` is called with body
  `{"column":"todo"}`
- **THEN** the response body's `card.column` equals `"todo"`

#### Scenario: Card is not archived

- **WHEN** `POST /api/cards/a3f2k9/unarchive` is called for a card that
  is currently live, not archived
- **THEN** the response status is `404`
- **AND** the body's `error.code` is `CARD_NOT_ARCHIVED`

#### Scenario: Id collision refuses without mutating either file

- **WHEN** `POST /api/cards/a3f2k9/unarchive` is called while a live
  card already has id `a3f2k9`
- **THEN** the response status is `409`
- **AND** the body's `error.code` is `ID_COLLISION`
- **AND** neither `kanban.toml` nor `kanban.archive.toml` changes on
  disk

### Requirement: `POST /api/columns/{name}/archive` archives a column's cards

`POST /api/columns/{name}/archive` SHALL archive every card in the
named column, cascading to epic children living in other columns
exactly as the CLI's `ezida archive column <name> --yes` does (this
route has no interactive prompt, so it always behaves as if `--yes`
were passed — the viewer's own confirm dialog, if any, happens
client-side before the request), and respond `200` with
`{"archived":[...],"cascaded":[...]}`, both `[]` rather than `null`
when empty. The column itself MUST remain in `[board].columns`. An
unknown column MUST respond `400 COLUMN_NOT_FOUND` — the same status
every other mutation route uses for this code via the shared
`errors.As` error-mapping chain; only `POST /api/cards`'s column check
is the established 404 departure, and this route is not it. An empty
column MUST respond `200` with both arrays empty and MUST NOT create
an archive file if none exists.

#### Scenario: Archiving a column's cards

- **WHEN** `POST /api/columns/done/archive` is called against a column
  holding four cards, none of which cascade elsewhere
- **THEN** the response status is `200`
- **AND** the body's `archived` array has length 4
- **AND** `[board].columns` in a subsequent `GET /api/board` still
  contains `done`

#### Scenario: Cascade reaches cards outside the column

- **WHEN** `POST /api/columns/done/archive` is called and an epic in
  `done` has a child in `todo`
- **THEN** the response body's `cascaded` array names that child

#### Scenario: Unknown column

- **WHEN** `POST /api/columns/ghost/archive` is called
- **THEN** the response status is `400`
- **AND** the body's `error.code` is `COLUMN_NOT_FOUND`

#### Scenario: Empty column is a no-op

- **WHEN** `POST /api/columns/review/archive` is called against a
  column with no cards
- **THEN** the response status is `200`
- **AND** both `archived` and `cascaded` are `[]`
- **AND** `kanban.toml` is byte-unchanged on disk
