## Why

The viewer renders every priority badge with the same neutral chip color. Users want priority to read at a glance — green/orange/red is the obvious convention, but priority names are configurable, so the mapping must live in `kanban.toml` rather than be hard-coded in CSS.

## What Changes

- Add optional `[board.priority_colors]` table to `kanban.toml`, mapping priority name → hex color (`#rgb` or `#rrggbb`).
- Extend board validation: every key must be a declared priority; every value must be a valid hex color. Unknown keys or invalid values fail `Validate` with a new rule.
- `GET /api/board` returns a new `priority_colors` object. The server resolves defaults for the conventional names `low` (`#22c55e`), `medium` (`#f59e0b`), `high` (`#ef4444`) — user values from the TOML always win.
- Viewer (`internal/server/web/`) and demo (`site/demo/`) apply the resolved color as inline `background-color` (and matching border) on the `.badge` element. Priorities without a color keep the current neutral chip skin.
- Demo `board.json` snapshot regenerated to include `priority_colors`.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `board-storage`: `BoardConfig` schema gains optional `priority_colors`; new validation rule for it.
- `viewer-server`: `/api/board` JSON shape adds `priority_colors` map with server-resolved defaults.
- `viewer-ui`: priority badge applies the per-priority color from `priority_colors`.

## Impact

- Code: `internal/board/board.go`, `internal/board/validation.go`, `internal/server/handlers.go`, `internal/server/web/{index.html,app.js}`, `site/demo/{index.html,app.js,board.json}`.
- Wire: `/api/board` response gains `priority_colors` (additive, backward-compatible).
- Schema: still `schema_version = 1` — the new field is optional.
- No CLI surface change in this round (TOML hand-edit is sufficient per the source card).
