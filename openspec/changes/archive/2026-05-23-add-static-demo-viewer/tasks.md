## 1. `ezida export --json` CLI command

- [x] 1.1 Add `internal/commands/export.go` defining `NewExportCmd(jsonOut *bool)` that loads the board and emits the same JSON envelope as `GET /api/board`. Mirror `project_name` resolution (parent-dir of the board path).
- [x] 1.2 Add `internal/commands/export_test.go` covering: full envelope shape, `project_name` from parent dir, empty `cards: []` and `tags: []`, error on missing `kanban.toml`.
- [x] 1.3 Register the command in `cmd/ezida/main.go`.
- [x] 1.4 Run `go test ./...` — all green.

## 2. Demo viewer assets in `site/demo/`

- [x] 2.1 Create `site/demo/index.html` as a copy of `internal/server/web/index.html` with two additions: (a) the line `<script src="demo-shim.js"></script>` immediately before `<script src="app.js"></script>`, (b) a `<div class="demo-banner">Demo — snapshot __SNAPSHOT_SHA__ · changes don't persist</div>` element at the top of `<body>`.
- [x] 2.2 Create `site/demo/demo-shim.js` that: (a) fetches `board.json` once at load, (b) overrides `window.fetch` to route `/api/*` against an in-memory state, (c) overrides `window.EventSource` with a no-op that fires `open` once. Cover all 9 endpoints listed in design D6.
- [x] 2.3 Create symlinks: `site/demo/app.js → ../../internal/server/web/app.js`, `site/demo/style.css → ../../internal/server/web/style.css`, `site/demo/vendor → ../../internal/server/web/vendor`.
- [x] 2.4 Add minimal `.demo-banner` CSS to the inline style block of `site/demo/index.html` (sticky top bar, accent background, monospace, ~36px high).
- [x] 2.5 Add `site/demo/board.json` to `.gitignore`.

## 3. Pages workflow extension

- [x] 3.1 In `.github/workflows/pages.yml`, add `kanban.toml` to the `on.push.paths` list.
- [x] 3.2 Insert before `actions/upload-pages-artifact@v3`: `actions/setup-go@v5` (Go 1.22), `go build -o ezida ./cmd/ezida`, `./ezida export --json > site/demo/board.json`.
- [x] 3.3 Insert a `sed -i "s/__SNAPSHOT_SHA__/${GITHUB_SHA:0:7}/g" site/demo/index.html` step followed by `! grep -q __SNAPSHOT_SHA__ site/demo/index.html` sentinel check.

## 4. Landing page demo links

- [x] 4.1 In `site/index.html` `#webui` section, append a "Try the live demo →" link below the figcaption pointing to `demo/`.
- [x] 4.2 In `site/index.html` `#roadmap` section, add a second sentence linking to `demo/` ("Or try the live demo → ").

## 5. Verification

- [x] 5.1 Run `go test ./...` — green.
- [x] 5.2 Locally: `go build -o ezida ./cmd/ezida && ./ezida export --json > site/demo/board.json && python3 -m http.server 8765 --directory site`, open `/demo/` in Chrome, verify cards render and a drag visually moves a card.
- [x] 5.3 Verify the `demo-banner` shows `dev` locally (placeholder substitution skipped) and that the page does not 404 on `/demo/app.js` (symlinks resolve).
