## Context

The viewer (`ezida serve`) is a Go HTTP server with embedded HTML/CSS/JS at `internal/server/web/`. The frontend (`app.js`, ~727 lines, Alpine.js) makes 9 distinct `/api/*` calls: 1 GET (`/api/board`), 1 SSE stream (`/api/events`), and 7 mutation endpoints (POST/PATCH/DELETE on `/api/cards/*` and `/api/columns/*`). The landing page (`site/`) is a static GitHub Pages deployment with no JS framework.

We want to ship a "try-it" demo at `https://nicolasvergoz.github.io/ezida-kanban/demo/` that lets visitors interact with the viewer using Ezida's own `kanban.toml` as fixture data, without installing anything. Pages is static, so the demo must be 100% client-side and mutations must be ephemeral.

## Goals / Non-Goals

**Goals:**
- Visitors can drag cards, edit titles, rename columns, switch theme — the full UX of the real viewer.
- The demo board reflects the current state of Ezida's own `kanban.toml` at deploy time.
- Zero duplication of viewer code in git: `app.js`, `style.css`, `vendor/` are symlinked from `internal/server/web/`.
- A board-only commit (changes only `kanban.toml`) triggers a Pages redeploy so the demo updates.

**Non-Goals:**
- Persistence of demo edits (in-memory only; refresh resets state).
- A "Reset demo" button (V1 — F5 is enough).
- PR previews of demo state.
- Multi-tab sync (no SSE → no live updates).
- Bundling a TOML parser in JS (we generate JSON in CI instead).
- Modifying the existing Go server or viewer code.

## Decisions

### D1. fetch+EventSource monkey-patch shim, not ServiceWorker, not in-app flag

Three architectures were considered:

| Approach | app.js touched | New code | Demo state realism | Risk |
|---|---|---|---|---|
| Service Worker | 0 | ~200 lines SW | full | SW lifecycle, scope pitfalls |
| **fetch+ES shim** | **0** | **~150 lines** | **full** | **monkey-patch surface** |
| Demo flag in app.js | 9+ sites | ~100 lines | full | drift as endpoints grow |

The shim wins because (a) `app.js` is untouched — production viewer and demo share one file via symlink, no drift risk, (b) the entire viewer-server API surface is concentrated in `app.js` via 9 `fetch()` calls and 1 `EventSource`, so a single point of interception covers everything, (c) graceful fallback: if a future endpoint is added without shim coverage, it 404s and `app.js`'s existing error path refetches `/api/board` (which the shim does cover).

### D2. JSON snapshot, not client-side TOML parsing

The viewer's `app.js` consumes the JSON shape of `GET /api/board`, not TOML. Two ways to feed it:

- **Symlink `kanban.toml` into `site/demo/`, parse client-side**: needs a JS TOML lib (~6KB smol-toml) and a re-implementation of `handleBoard` shape logic (computes `project_name`, `cards_per_column`, fills empty arrays). Drift between TOML shape and `boardResponse` is the silent failure mode.
- **CI generates `board.json` via `ezida export --json`**: same Go struct that powers `/api/board` → same shape, by construction. No drift possible. Costs one new CLI command.

We pick the CI route. Bonus: `ezida export --json` is genuinely useful as a CLI primitive beyond the demo (scripting, integrations, snapshot diffing in PRs).

### D3. Symlinks for shared viewer assets

`site/demo/` needs `app.js`, `style.css`, `vendor/` — same files the real viewer uses. Three options:

- **Copy at commit time** → manual sync, drift the moment someone forgets.
- **Copy at CI deploy time** → no drift on Pages, but the demo can't be previewed locally without running a build step.
- **Symlink** → one source of truth, works locally, `actions/upload-pages-artifact@v3` resolves symlinks at tar time (verified: `actions/toolkit` artifact uploader follows symlinks).

We pick symlinks. Solo dev on macOS/Linux — symlinks are a non-issue. If Windows contributors ever join, the CI copy approach can replace this without changing the demo's external interface.

### D4. Banner with snapshot SHA

The banner reads `Demo — snapshot <sha7> · changes don't persist` where `<sha7>` is the first 7 chars of `$GITHUB_SHA`. Substituted by a `sed` step in CI directly into `site/demo/index.html` before upload.

Locally (no CI), the banner shows `Demo — snapshot dev · changes don't persist`. Dev sentinel makes the local-vs-deployed distinction visible without breaking anything.

### D5. Trigger pages.yml on kanban.toml changes

Adding `kanban.toml` to the workflow's `paths` filter means every board edit (move card, add tag, rename column) triggers a Pages redeploy. This is the user's stated goal. Side effect: deploys-per-day goes up. Acceptable — solo dev, ~5–10 board edits/day max, Pages is generous.

### D6. Shim implementation: in-memory router

```
demo-shim.js
├── async load board.json
├── window.fetch = async (input, init) => {
│     const url = String(input);
│     if (!url.startsWith('/api/')) return realFetch(input, init);
│     return route(url, init?.method || 'GET', init?.body);
│   }
├── window.EventSource = class { ... }  // no-op, emits 'open' once
└── route(url, method, body):
      GET    /api/board                   → JSON.stringify(state)
      GET    /api/events                  → 204 (never reached, EventSource overridden)
      POST   /api/cards/{id}/move         → state.cards[id].column = body.to
      PATCH  /api/cards/{id}              → merge body into state.cards[id]
      DELETE /api/cards/{id}              → splice
      POST   /api/columns                 → state.columns.push(body.name)
      PATCH  /api/columns/{old}           → rename column + cascade to cards
      DELETE /api/columns/{col}           → splice (refuse if cards reference it)
      POST   /api/columns/move            → reorder state.columns
      *                                   → 404, app.js refetches gracefully
```

All routes return the same JSON shape as the real server. Validation rules mirror server-side checks (refuse delete of non-empty column, refuse duplicate column names, etc.) so the UI's error display works identically.

## Risks / Trade-offs

- **Shim drift**: if a future server response shape changes (e.g. add a field to a card), `app.js` may rely on it and the shim won't know. Mitigation: the shim returns whatever shape `board.json` has, and `board.json` is generated by the same Go code that backs `/api/board` — so server-side shape changes propagate to the demo automatically. The only risk is shim *mutation* responses (PATCH responses) — for those, the shim re-serializes the local state, which uses the same field set. If new fields appear, the shim's mutation responses will be missing them; the worst case is a UI field that doesn't visibly update until F5.
- **Symlinks + upload-pages-artifact**: relies on the action resolving symlinks at tar time. Documented as supported, but if the behavior changes the demo's shared assets would 404. Mitigation: a smoke-test step (`curl /demo/app.js` after deploy) can be added later if it ever becomes flaky.
- **Banner SHA substitution via sed**: simple but coupled to the exact HTML markup. If `index.html` is reformatted such that the placeholder `__SNAPSHOT_SHA__` no longer matches, the build silently leaves the placeholder visible. Mitigation: a sentinel check (`grep -q __SNAPSHOT_SHA__ site/demo/index.html && exit 1`) after the sed step.
- **In-memory state across mutations**: `app.js` was written assuming a real server. Some endpoints return the updated card (PATCH /api/cards/{id}); some return only 204 (POST move). The shim must match each endpoint's documented response shape exactly. Reading the relevant `handlers.go` sections is non-negotiable.
- **Bot/scraper noise**: a demo at a public URL gets indexed and crawled. Acceptable — robots.txt already allows the landing page, demo follows suit.
