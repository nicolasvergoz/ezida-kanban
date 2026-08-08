# Show epics in the viewer: wire fields and read-only rendering

## Why

`add-epics-schema-cli` puts epics on disk and in the CLI, but the viewer — the surface most people actually look at — renders nothing. A board can carry six cards grouped under one parent and look exactly like a board with six unrelated cards.

That change also left a deliberate inconsistency: `ezida get --json` exposes `epic` while `ezida export` does not, because `output.ExportCard` and `server.cardResponse` are parallel structs that must move together. This change closes it.

Scope is read-only rendering. Nothing here creates, edits, or removes an epic relation; the board displays what the CLI already writes.

**Depends on `add-epics-schema-cli`.** `board.Card` must carry `Epic` and `Color`, and `BoardConfig` must expose terminal columns, before any of this is buildable.

## What Changes

**Wire — two fields, no denormalization.** `cardResponse` gains `epic` and `color`, both `omitempty`. `boardResponse` gains `done_columns`. Nothing else.

Specifically, the wire does **not** carry `epic_title`, `epic_color`, `children`, or `progress`. `GET /api/board` already returns every card on the board, so the client has everything it needs to resolve a parent by id and count children itself. Denormalizing would grow the payload, duplicate truth, and create a second thing to keep correct on every mutation. `ezida get --json` resolves the parent's title because it returns a single card with no board around it; the board endpoint has no such excuse.

**`ezida export` follows.** `output.ExportCard` gains the same two fields and the export envelope gains `done_columns`, restoring the "same shape as `GET /api/board`" contract that `board-export` states.

**Child cards show a colored chip.** In the existing `.card-foot` row, first element after the priority pill, before the tags. Coloured from the parent's `color`; tags stay grey, which is what separates the two — no new shape vocabulary. Truncated at a fixed max-width with the full name in a `title` attribute.

**Parent cards show they are parents.** A four-square glyph before the title, a border tinted with the card's own colour, and a progress bar with a `done/total` counter. All three appear only on a card that is actually referenced by another — a card carrying a `color` but no children renders exactly as it does today.

**Terminal columns are marked.** A small check between the list title and the count, reusing the existing `IconCheck`. This is what makes the progress counter legible: without it, a user cannot tell which column feeds the numerator.

**The modal reports the relation, read-only.** A child shows a row naming its parent. A parent shows its children with their columns, plus the progress bar. No picker, no colour swatches, no assignment — that is the next change.

**A single hex renders in both themes.** The chip never uses the stored hex as a text colour directly; it mixes it toward the current theme's `--text` through `color-mix`, exactly as `.card-tag-chip` already does. One stored value, two legible renderings, nothing theme-specific in the file.

**Explicitly out of scope.** No editing of the epic relation or the colour. No epic scope in the filter popover. Both land in later changes.

## Capabilities

### New Capabilities
<!-- None. Every change extends an existing viewer or export capability. -->

### Modified Capabilities
- `viewer-server`: `GET /api/board` gains `epic` and `color` per card and a top-level `done_columns`; the envelope's `schema_version` is now `2` and the version-mismatch scenario shifts accordingly.
- `board-export`: the export envelope gains the same fields, keeping it shape-identical to `GET /api/board`.
- `viewer-ui`: the wire↔UI adapter carries the new fields and resolves parents by id; new rendering requirements for the epic chip, the parent card, the terminal-column marker, and the modal's read-only relation sections.

## Impact

**Code**
- `internal/server/handlers.go`: `cardResponse`, `boardResponse`, `cardToResponse`, `handleBoard`.
- `internal/output/json.go` and `internal/commands/export.go`: `ExportCard` and the export envelope.
- `internal/server/web/app.jsx`: `toUiBoard` (carry `epic`/`color`, build an id→card index), the card component, the list header, the detail modal.
- `internal/server/web/styles.css`: the chip, the parent-card treatment, the progress bar, the terminal marker.

**Inherited for free**: `site/demo/app.jsx` and `site/demo/styles.css` are symlinks to the embedded assets, so the demo picks up the rendering without edits. `site/demo/board.json` is a snapshot and must be regenerated with `ezida export` for the demo to actually show an epic.

**Untouched**: `internal/board/` and every CLI command. This change reads what the previous one wrote.

**No breaking change.** Both new card fields are `omitempty`, so a board with no epics produces a byte-identical payload to today's. Every existing viewer test that asserts on the envelope keeps passing without modification.

**Docs**: `docs/usage.md`'s "What the Web UI lets you do" list gains the epic-rendering entries.
