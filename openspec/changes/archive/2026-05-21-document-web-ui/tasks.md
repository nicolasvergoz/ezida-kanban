## 1. README

- [x] 1.1 Add a "Web UI" sub-section to `README.md` after the CLI
      examples block (around line 53), before the "Documentation"
      section. Content: one short paragraph positioning the Web UI
      as the visual companion to the CLI (read + write, hot-reloads
      on file change, loopback-only). One fenced block showing
      `ezida serve`. One sentence linking to the full reference in
      `docs/usage.md`.

## 2. docs/usage.md

- [x] 2.1 Add `### ezida serve` reference section to
      `docs/usage.md`, placed at the end of the "CLI reference"
      block (after `### ezida priorities`, before "## JSON
      contract"). Content:
      - One-line summary (HTTP server on 127.0.0.1, defaults to
        port 7777, fallback +10).
      - Flag table: `--port`, `--no-open`.
      - One fenced example.
      - A short "What the Web UI can do" bullet list covering the
        shipped capabilities: read the board, inline create / edit
        / delete cards, drag-and-drop card reorder and move
        between columns, inline column add / rename / delete /
        reorder, board filter, dark theme, SSE hot reload of
        `kanban.toml`.
      - Pointers to `openspec/specs/viewer-server/spec.md` and
        `openspec/specs/viewer-ui/spec.md` for the authoritative
        behaviour spec.

## 3. Verify

- [x] 3.1 Re-read the updated `README.md` and `docs/usage.md`
      end-to-end. Confirm: every flag named matches
      `internal/commands/serve.go`; the capability list matches
      what `internal/server/handlers.go` + `internal/server/web/`
      actually serve; no broken in-doc links.
- [x] 3.2 Run `openspec validate document-web-ui --strict` and
      confirm it passes.
