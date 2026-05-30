## 1. Viewer filter state

- [x] 1.1 Add `inId: true` to `DEFAULT_FILTER` in `internal/server/web/app.jsx`

## 2. Match logic

- [x] 2.1 Extend `matchCard` in `internal/server/web/app.jsx` to test
  `(card.id || "").toLowerCase().includes(q)` when `f.inId` is true
- [x] 2.2 Update the early-exit "all scopes off" guard in `matchCard` to
  include `f.inId`

## 3. Filter popover UI

- [x] 3.1 Add an `ID` scope pill to the filter-pills row in the popover,
  bound to `inId`, using the same markup/handlers as the existing Title /
  Description / Tags pills

## 4. Verify with go build

- [x] 4.1 Run `go build ./...` and confirm exit 0 (the viewer assets are
  embedded via `embed.go`; this rebuilds the binary and catches syntax /
  embed errors)

## 5. Verify in browser

- [x] 5.1 Start `./ezida serve --port 8765 --no-open`, fetch
  `http://127.0.0.1:8765/static/app.jsx`, and confirm the served file
  contains the new `inId` defaults, `matchCard` branch, and `ID` pill
  markup
