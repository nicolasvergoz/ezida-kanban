## Context

The viewer (`internal/server/web/app.jsx`) loads the board once on
mount via `fetchBoard` (app.jsx:314), then keeps it fresh through an
SSE subscription (`/api/events`, app.jsx:329-343) that refetches on
`board-changed`. When the SSE connection is down, no path exists to
re-fetch on demand — the user must reload the whole page.

`TopBar` (app.jsx:560) already receives the pieces needed to add a
control: it renders the filter button on the left of its right zone,
then a divider, then `ThemeToggle` and `ServerStatus` (app.jsx:668-672).
Icons are declared as one-liners over a shared `Icon` component
(app.jsx:214-227). Because the app is bundled inline with Babel and
served from `internal/server/web/app.jsx` → `app.js` (checked in),
every viewer change is a code edit to `app.jsx` plus `styles.css`, and
the compiled `app.js` is regenerated — headless-browser tests drive the
real server, so they cover the wire and the rendering together.

## Goals / Non-Goals

**Goals:**

- Manual, on-demand re-fetch of the board without a full page reload.
- Control always visible in the topbar right zone, to the right of the
  connection status.
- Visible busy state while the re-fetch is in flight; clicks ignored
  during flight.
- Reuses the existing fetch path, error handling, and re-render.

**Non-Goals:**

- Auto-refresh changes; SSE behaviour stays as-is.
- Server/API changes — no wire-shape or handler edits
  (`ExportsWantsBoard`, `cardResponse`, etc. untouched).
- Configurable placement, keyboard shortcuts, or a reload spinner
  component library.

## Decisions

### D1. Reuse `fetchBoard`, add a `refreshing` flag — no `location.reload()`

The existing `fetchBoard` (a `useCallback`) is the single load path:
initial load, SSE refetch, and mutation refetches all go through it.
The refresh button calls the same function wrapped in an in-flight
guard.

In `App`:

```js
const [refreshing, setRefreshing] = useState(false);

const refreshBoard = useCallback(async () => {
  if (refreshingRef.current) return;
  refreshingRef.current = true;
  setRefreshing(true);
  try { await fetchBoard(); }
  finally {
    refreshingRef.current = false;
    setRefreshing(false);
  }
}, [fetchBoard]);
```

A ref (rather than reading the state) guarantees the guard is atomic
against double-clicks within the same tick, and the state drives the
busy UI. `fetchBoard` already sets `board` and clears `loadError` on
success and stores `loadError` on failure — error feedback comes for
free, matching the initial-load path.

### D2. Thread `onRefresh` + `refreshing` into `TopBar`, render an `iconbtn`

`TopBar` gains two props and one button, inserted after `ServerStatus`
(app.jsx:672):

```jsx
<ServerStatus status={sseStatus} />
<button
  className={"iconbtn" + (refreshing ? " refreshing" : "")}
  onClick={onRefresh}
  disabled={refreshing}
  aria-label="Refresh"
  title="Refresh">
  <IconRefresh />
  <span className="iconbtn-label">Refresh</span>
</button>
```

Matches the existing filter `iconbtn` pattern (app.jsx:588-596) and
keeps the "right zone = connection + refresh" grouping. `disabled`
during flight enforces the click guard at the DOM level.

### D3. `IconRefresh` as a shared-icon one-liner

Feather-style spinning-arrows path, alongside the other icons
(app.jsx:216-227):

```jsx
const IconRefresh = (p) => <Icon {...p} d={<polyline points="23 4 23 10 17 10" /><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />} />;
```

### D4. Busy state is a CSS spin on the icon

`.iconbtn.refreshing svg { animation: spin 0.9s linear infinite; }`
with a `@keyframes spin` rule. Token-driven styling like the rest of
`styles.css`; no hex literals outside `:root` (per the tokens
requirement), no JS animation library.

### D5. e2e: assert button presence and that a click issues `GET /api/board`

SSE is connected throughout a normal e2e test, so asserting "the
board changed" can't isolate the button. Instead the test counts
`GET /api/board` requests on the page: record the count before the
click, click, assert the count incremented by exactly one within the
debounce window, using an in-page `request` listener on
`page.on("request")`. This pins the button to the fetch without
depending on SSE at all. A second assertion in the same spec covers
the always-visible behaviour by killing the SSE connection (close the
`EventSource` via `page.evaluate`) and confirming the button is still
present and enabled.

## Risks / Trade-offs

- [Double-fetch race with the SSE debounced refetch] → Both paths
  funnel through `fetchBoard`; a concurrent refetch is idempotent
  (`fetchBoard` just overwrites `board`). The `refreshing` guard only
  suppresses a *user-initiated* second click.
- [Test flakiness from timing] → Count requests rather than asserting
  rendered change; poll until the count increments with a short
  timeout, mirroring the existing e2e helpers' robust style.
- [`refreshingRef` vs plain state guard] → A ref keeps the guard atomic
  within one tick; state alone could let two clicks in a single frame
  through. Ref + state pair is small and already a pattern in `App`
  (`refetchTimer` ref alongside state).
- [Busy state adds a one-off CSS animation] → Encapsulated as a single
  keyframe + class; consistent with Redacto's small-decoration style.
- [Browser tests need a live query count] → `page.on("request")`
  counts all board fetches; the test diff (`after` − `before`) isolates
  the click's contribution, so concurrent SSE refetches inflate both
  sides equally and cannot falsify the +1 assertion against a still
  higher total.

## Migration Plan

None — additive viewer change. Deployment is the normal serve path;
the compiled `app.js` is regenerated and committed with `app.jsx`.

## Open Questions

None.
