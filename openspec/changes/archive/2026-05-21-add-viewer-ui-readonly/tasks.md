## 1. Vendor Alpine.js

- [x] 1.1 Pick the latest Alpine 3.x stable release at implementation time and record the version in a leading comment of `internal/server/web/vendor/alpine.min.js`. Done when the file exists and begins with `/* Alpine.js v3.<minor>.<patch> - https://alpinejs.dev */`.
- [x] 1.2 Download `https://cdn.jsdelivr.net/npm/alpinejs@<version>/dist/cdn.min.js` into the file. Done when the file size matches the vendor's published bundle (±50 bytes for the leading comment) and `GET /static/vendor/alpine.min.js` returns the contents.

## 2. Stylesheet

- [x] 2.1 Replace `internal/server/web/style.css` with the minimal layout per design. Done when the file contains rules for `.topbar`, `.columns`, `.column`, `.card`, `.priority-low|medium|high`, `.tag`, `.empty`, `.loading` selectors and weighs < 4 KB.

## 3. Alpine component

- [x] 3.1 Replace `internal/server/web/app.js` with the `board()` function from design (load + cardsByColumn). Done when the file ends with `if (window) window.board = board;` (or equivalent global wiring) and contains no `import`/`export`.

## 4. HTML page

- [x] 4.1 Replace `internal/server/web/index.html` with the structured page per design (topbar + Alpine root, no inline styles). Done when the rendered DOM under `GET /` contains `header.topbar`, `main[x-data]`, the empty-state template, and references to `/static/style.css`, `/static/vendor/alpine.min.js`, `/static/app.js`.

## 5. Server-side sanity

- [x] 5.1 Confirm `internal/server/handlers.go` already serves `/static/vendor/*` via the existing FileServerFS — no code change expected. Done when `curl -I http://127.0.0.1:<port>/static/vendor/alpine.min.js` returns `200 OK` with `Content-Type: text/javascript` (or `application/javascript`).
- [x] 5.2 Confirm `GET /` Content-Type is `text/html; charset=utf-8`. If V1 server set only `text/html`, add the `charset=utf-8` explicitly. Done when the response header matches.

## 6. Tests

- [x] 6.1 Add `TestStatic_Vendor_Alpine` in `internal/server/server_test.go` asserting `GET /static/vendor/alpine.min.js` returns 200 and the body starts with the vendored comment line. Done when the test passes.
- [x] 6.2 Add `TestIndex_References_VendoredAssets` asserting the body of `GET /` contains the substrings `/static/vendor/alpine.min.js`, `/static/app.js`, `/static/style.css`. Done when the test passes.
- [x] 6.3 Add `TestIndex_NoExternalScripts` asserting the body of `GET /` does NOT contain `https://` or `http://` inside `src=` attributes. Done when the test passes.

## 7. Browser smoke

- [x] 7.1 Run `./ezida serve` in a project with at least 2 columns and 3 cards spanning multiple priorities. Visually confirm columns render horizontally, cards stack vertically inside each column, priority shows on the left edge of cards, tag chips render, empty columns show the placeholder. Done when the developer signs off on the manual check (recorded as a comment in the change after validation). — Verified via Chrome MCP smoke test: 5 cols horizontal, cards stack vertical inside, 3px left border per priority (high/medium/low distinct shades), tag chips render, empty column shows "empty". Required script-order fix in index.html (app.js before alpine.min.js so window.board is defined when Alpine evaluates x-data).

## 8. Acceptance gate

- [x] 8.1 Run `go test ./... && go vet ./...`. Done when exit code is 0 and the new tests appear in the output.
