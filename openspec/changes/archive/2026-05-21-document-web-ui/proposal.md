## Why

The `ezida serve` command and its accompanying interactive Web UI
shipped over the last few changes (viewer-server, viewer-ui, hot
reload, inline edits, inline column ops, modal, dark theme, board
filter, tokens/chrome redesign) — but the user-facing docs were
never updated. A new user reading `README.md` or `docs/usage.md`
has no way to discover that `ezida` ships a browser-based Web UI
at `http://127.0.0.1:7777`, or what it can do (read + write,
hot reload, drag-and-drop, inline edits, dark mode, filter).

## What Changes

- Add a "Web UI" sub-section to the README's Quick start, after the
  CLI examples — short paragraph + one command (`ezida serve`)
  positioning the page as the visual companion to the CLI.
- Add a full `### ezida serve` CLI-reference section to
  `docs/usage.md`, alongside `add`/`edit`/`move`/etc., covering
  flags (`--port`, `--no-open`), default port + fallback window,
  loopback-only bind, and the Web UI capabilities currently
  shipped (read, inline create/edit/delete, drag-reorder,
  per-column ops, filter, dark theme, hot reload).
- No code changes. Docs-only.

## Capabilities

### New Capabilities

- `documentation`: lightweight capability that captures normative
  requirements about user-facing docs (`README.md`, `docs/usage.md`).
  Today it asserts that the Web UI shipped via `viewer-server` /
  `viewer-ui` is surfaced to readers; future doc-only changes can
  extend it with further "the docs SHALL mention X" requirements.

### Modified Capabilities

(none — no behavioural code-level spec changes. Existing specs at
`openspec/specs/viewer-server/` and `openspec/specs/viewer-ui/`
already describe the runtime behaviour and are unchanged here.)

## Impact

- `README.md` — adds a short Web UI sub-section.
- `docs/usage.md` — adds a `### ezida serve` reference section.
- No source code, no tests, no dependencies touched.
- No version bump required (docs only).
