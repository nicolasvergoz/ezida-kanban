## Why

The viewer already refetches `/api/board` when the SSE stream pushes a
`board-changed` event, but that auto-refresh depends on a live
`EventSource`. When SSE is offline — connection dropped, server
restarted, or the watcher missed an external edit — the only way to
get fresh data is a full page reload. There is no button to re-fetch
on demand.

## What Changes

- Add a refresh action to the viewer topbar, placed immediately to the
  right of the connection status.
- The button re-fetches the board in place via the existing
  `/api/board` load path — no full page reload (`location.reload` is
  never invoked).
- The button is always visible, regardless of SSE status; it is the
  manual fallback whenever the auto-refresh path is unavailable.
- While a refetch is in flight the button shows a busy (spinning)
  affordance and ignores further clicks.
- The viewer's data-loading behaviour itself is unchanged: the refresh
  button reuses the same fetch, error handling, and re-render path as
  the initial load and the SSE-driven refetch.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `viewer-ui`: a new requirement — the topbar right zone gains a
  manual refresh control next to the connection status. The existing
  auto-refresh-on-SSE behaviour is unchanged.

## Impact

- `internal/server/web/app.jsx` — `App` tracks a `refreshing` flag
  around `fetchBoard`; `TopBar` receives `onRefresh` and `refreshing`
  props and renders the refresh `iconbtn` + `IconRefresh`.
- `internal/server/web/styles.css` — spinner styles for the refresh
  button's busy state (token-driven, no hard-coded hex).
- `e2e/` — a new spec asserts the button exists and that clicking it
  issues an additional `GET /api/board` request (no dependency on the
  SSE path).
- No server-side, API, wire-shape, or backend changes.
