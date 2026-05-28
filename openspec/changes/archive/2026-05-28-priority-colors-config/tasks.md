## 1. Board schema + validation

- [x] 1.1 Add `PriorityColors map[string]string` (toml tag `priority_colors,omitempty`) to `BoardConfig` in `internal/board/board.go`
- [x] 1.2 Add rule 10 to `Validate` in `internal/board/validation.go`: keys ⊆ `Priorities`, values match `^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`
- [x] 1.3 Add unit tests in `internal/board/board_test.go` (round-trip with and without `priority_colors`)
- [x] 1.4 Add unit tests for rule 10 (unknown key, bad hex value, valid map, empty map) — added in `internal/board/board_test.go` since the project keeps validation tests there, not in a separate `validation_test.go`

## 2. /api/board response

- [x] 2.1 Add `PriorityColors map[string]string \`json:"priority_colors"\`` to `boardResponse` in `internal/server/handlers.go`
- [x] 2.2 Implement default resolution helper: low=`#22c55e`, medium=`#f59e0b`, high=`#ef4444` — only fill when priority is declared and user did not supply. Placed in `internal/board` as `ResolvePriorityColors` so the export command can reuse it (also extended to `output.ExportEnvelope` and `internal/commands/export.go` for the same wire shape).
- [x] 2.3 Call helper in `handleBoard`, populate `priority_colors` (always non-nil; empty `{}` when nothing resolves)
- [x] 2.4 Add handler tests in `internal/server/server_test.go` covering: defaults filled, user override wins, custom priority returned, custom priority without color absent

## 3. Viewer (embedded UI)

- [x] 3.1 Read `data.priority_colors` in `internal/server/web/app.js` `load()` and store on `priorityColors` field (init `{}`)
- [x] 3.2 Bind `:style` on the `.badge` priority chip in `internal/server/web/index.html` to apply `background-color` + `border-color` when color present
- [x] 3.3 No error/disconnect path resets viewer state today — leaving `priorityColors` to be re-populated on each successful `load()` matches existing behavior

## 4. Demo viewer

- [x] 4.1 `site/demo/app.js` is a symlink to `internal/server/web/app.js`; 3.1 already covers it
- [x] 4.2 Mirrored badge `:style` binding in `site/demo/index.html`; also taught `site/demo/demo-shim.js` to defensively coerce `priority_colors` and added defaults to its fallback state
- [x] 4.3 Regenerated `site/demo/board.json` via `ezida export --json` so it now includes the resolved `priority_colors` field

## 5. Verify

- [x] 5.1 `go build ./...` succeeds
- [x] 5.2 `go test ./...` passes
- [x] 5.3 Added `[board.priority_colors]` entry to project `kanban.toml` with the three defaults; `ezida list --json` still loads the file cleanly
