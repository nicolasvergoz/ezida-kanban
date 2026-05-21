## Context

V1 server (in `add-viewer-server-skeleton`) embeds `web/` and serves
its contents via `/` and `/static/*`. The placeholder `index.html`
proves the wiring works; this phase ships the actual page. All
cross-cutting choices — vendoring strategy, no build step, no CDN —
are pinned in ADR 0002 §D5.

The user explicitly asked to keep visual design minimal in this phase
("le design se fera après reste minimaliste pour le moment"). This
design honors that: a structurally complete board, sized and laid
out, but no theming, no animations, no decoration beyond what's
needed to tell columns from cards.

## Goals / Non-Goals

**Goals:**
- A working read-only Kanban board visible at `http://127.0.0.1:<port>`.
- Columns rendered horizontally (scroll horizontally if many).
- Cards stacked vertically inside each column.
- Each card shows title, priority badge (if set), tag chips, and an
  updated-at tooltip on hover.
- Each column header shows the column name and a card-count badge.
- Empty columns display a placeholder string.
- Page is keyboard-accessible to the extent stock HTML provides
  (semantic elements, no `tabindex` tricks needed in V1).

**Non-Goals:**
- No drag and drop. No `Sortable.js` vendored yet (V2).
- No card edit modal. No event handlers beyond `x-init` fetch (V3).
- No SSE client. The page only knows about the board at load time;
  external changes require a manual refresh (V4 fixes this).
- No theming, no dark mode, no custom fonts.
- No mobile-responsive layout.
- No icons, no images, no SVG sprites.
- No browser-side routing.

## Decisions

### Page structure

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Ezida</title>
  <link rel="stylesheet" href="/static/style.css">
  <script defer src="/static/vendor/alpine.min.js"></script>
  <script defer src="/static/app.js"></script>
</head>
<body>
  <header class="topbar">
    <span class="project-name" x-data x-text="document.title.startsWith('Ezida') ? 'Ezida' : document.title"></span>
  </header>
  <main x-data="board()" x-init="load()">
    <template x-if="!loaded"><div class="loading">Loading…</div></template>
    <div class="columns" x-show="loaded">
      <template x-for="col in columns" :key="col">
        <section class="column">
          <header class="column-header">
            <span class="column-name" x-text="col"></span>
            <span class="column-count" x-text="cardsByColumn(col).length"></span>
          </header>
          <ul class="cards">
            <template x-for="card in cardsByColumn(col)" :key="card.id">
              <li class="card" :class="card.priority ? 'priority-' + card.priority : ''" :title="'updated ' + card.updated_at">
                <div class="card-title" x-text="card.title"></div>
                <template x-if="card.priority">
                  <span class="badge" x-text="card.priority"></span>
                </template>
                <ul class="tags" x-show="card.tags && card.tags.length">
                  <template x-for="tag in (card.tags || [])" :key="tag">
                    <li class="tag" x-text="tag"></li>
                  </template>
                </ul>
              </li>
            </template>
            <template x-if="cardsByColumn(col).length === 0">
              <li class="empty">empty</li>
            </template>
          </ul>
        </section>
      </template>
    </div>
  </main>
</body>
</html>
```

Single Alpine root for the board. The top-bar `x-data` is decorative
(can be removed if it adds noise). No `<noscript>` fallback — the
viewer requires JS to be useful.

### Alpine component

```js
// internal/server/web/app.js
function board() {
  return {
    loaded: false,
    schema_version: 0,
    columns: [],
    priorities: [],
    cards: [],
    async load() {
      const res = await fetch('/api/board');
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        console.error('failed to load board', err);
        return;
      }
      const data = await res.json();
      this.schema_version = data.schema_version;
      this.columns = data.columns || [];
      this.priorities = data.priorities || [];
      this.cards = data.cards || [];
      this.loaded = true;
    },
    cardsByColumn(name) {
      return this.cards.filter(c => c.column === name);
    },
  };
}
```

The function is registered globally (Alpine 3 picks up
`function board()` declared before Alpine boots because of the
`defer` attribute ordering). No build step, no module bundler.

### CSS — minimum viable

The stylesheet (~50 lines) defines:

- Reset: `* { box-sizing: border-box }`, `body { margin: 0; font-family: system-ui }`.
- Topbar: `display: flex; align-items: center; height: 40px; border-bottom: 1px solid #ddd; padding: 0 12px`.
- Columns wrapper: `display: flex; gap: 12px; padding: 12px; overflow-x: auto; min-height: calc(100vh - 40px)`.
- Column: `min-width: 260px; max-width: 320px; background: #f4f4f4; border-radius: 6px; padding: 8px`.
- Card: `background: #fff; border: 1px solid #ddd; border-radius: 4px; padding: 8px; margin-bottom: 6px; cursor: default`.
- Priority swatches via `.priority-low { border-left: 3px solid #999 }`, `.priority-medium { border-left: 3px solid #666 }`, `.priority-high { border-left: 3px solid #222 }`. Grayscale on purpose — color comes with the design pass.
- Tag chip: `display: inline-block; font-size: 11px; padding: 1px 6px; background: #eee; border-radius: 999px; margin-right: 4px`.
- Empty placeholder: `color: #999; font-style: italic; padding: 6px`.

No CSS variables, no custom properties, no fonts loaded externally.

### Vendored Alpine

Bundled as `internal/server/web/vendor/alpine.min.js`. Source:
the official Alpine 3 CDN URL pinned to a specific version (e.g.
3.14.x); copy the bytes into the repo and commit. The file
header includes a comment line noting the source URL and version.

No build step, no auto-update mechanism. Future Alpine bumps are
manual: re-download, replace bytes, commit.

### Topbar minimum

A 40 px-tall row with the project name (literal "Ezida" in V1 —
no fetch of the current directory name; that's polish). The
connection status indicator (green dot for SSE-connected) is
deferred to V4 since SSE itself is V4.

## Risks / Trade-offs

- **No build step + ES2017+ syntax everywhere**: the Alpine
  component uses `async/await` and arrow functions. Targets
  modern browsers only (Chrome/Safari/Firefox releases from the
  last 5 years). Acceptable for a developer tool.
- **CSP**: the page allows inline Alpine attributes (`x-init`,
  `x-data`). Standard Alpine usage. No nonces, no CSP header —
  localhost-only, single user.
- **No error UI**: if `/api/board` fails, the page logs to console
  and stays in "Loading…". V5 polish adds an error toast; V1
  minimalism lives with the silent failure.
- **No persistence of scroll position**: refresh resets the
  horizontal scroll. Acceptable in v1.

## Migration Plan

Not applicable. Replacing placeholder HTML with the real page; no
existing user state, no rollback target.

## Open Questions

- Alpine pinned version: pick the latest stable at implementation
  time (V1-UI task list captures this). Document the version in a
  comment at the top of `vendor/alpine.min.js`.
