## Context

UI-1 shipped the token system and the redesigned chrome; UI-2 added
the dark-theme overrides; UI-3 added the transient client-side
filter. Through all three, the only way to add or remove a card
remained the CLI. UI-4 closes that loop with two Redacto-native
affordances — the column-foot composer and the hover-revealed
delete button — and the two server endpoints they imply.

The phase deliberately preserves V3's click-to-edit modal: the
modal is the right shape for the card detail surface (ADR 0003 §D3)
and UI-5 will deepen it with inline click-to-edit fields. Mixing
modal redesign work into UI-4 would balloon the diff and obscure
the create / delete behaviour we actually want to validate.

The SSE pipeline (delivered in V5, encoded in ADR 0002 §D9) is the
foundational assumption that lets this phase skip optimistic
client-state mutation: every successful create / delete fires a
`board-changed` event that the existing `handleExternalChange` hook
turns into a fresh `/api/board` fetch. The server's response body
on POST / DELETE is therefore a *fallback*, not the primary state
delivery path — useful when SSE is degraded, but the client does
not depend on it.

## Goals / Non-Goals

**Goals:**

- Add a card to any column without leaving the board view.
- Delete a card without a confirmation step (hover-only affordance
  is the friction — ADR 0003 §D13).
- Keep the existing click-to-edit modal working untouched.
- Reuse every error code already minted in ADR 0001 §D8 / ADR 0003
  §D9 — no new wire codes.
- Survive a drag-in-progress on a card without firing a stray
  delete from a mouseup that lands on the × button.
- Stay within the Alpine + Sortable + vanilla-CSS stack (ADR 0003
  §D2). No new vendored assets, no build step.

**Non-Goals:**

- Card creation from anywhere except the column foot. No "create
  in column X" toolbar, no keyboard shortcut to create a card from
  an empty board surface.
- Multi-card create (paste-newlines-as-cards). The composer accepts
  a single title per submit.
- Delete confirmation, undo toast, or trash bin (ADR 0003 §D13).
- Editing description / priority / tags during creation. The
  composer accepts a title only; description, priority, tags
  default to empty / unset and can be edited via the existing
  modal flow afterwards. (The HTTP endpoint *does* accept those
  optional fields — see "Wire shape" — so CLI-style scripts and
  AI assistants can still create a fully-populated card in one
  request.)
- Optimistic UI. The composer locks (`submitting`) during the POST
  and unlocks only after the response. The board re-render
  happens via SSE, not by mutating local state.

## Decisions

### D1. POST shape: body-carried column, ID generated server-side

The endpoint is `POST /api/cards` (not `POST /api/columns/:name/cards`).
The destination column travels in the JSON body. Rationale: the
existing `POST /api/cards/:id/move` already encodes "card identity is
the ID in the path, all other location data is in the body". The
create endpoint mirrors that shape and avoids the URL-encoded-column
edge case (columns with spaces, dots, etc.).

The ID is generated server-side via `board.NewUniqueID(existing)`
where `existing` is the slice of current `c.ID` values. Client cannot
suggest an ID. Six-char IDs from `[0-9a-z]` (ADR 0001 §D7) collide at
~1 / 2 billion rate; the existing retry-10 loop in `NewUniqueID` and
the `ErrIDExhausted` sentinel handle the once-in-a-blue-moon case.
`ErrIDExhausted` is not in the spec'd error code namespace; it
surfaces as `IO_ERROR` 500 via the catch-all in `httpError`. This is
consistent with how `Save` failures surface today — both are server
internal conditions the client cannot remediate.

**Alternatives considered:**

- `POST /api/columns/:name/cards`: rejected — adds a second
  column-name escape codepath (the move endpoint already keeps
  column in the body) and breaks symmetry with `PATCH /api/cards/:id`.
- Let client pass an ID: rejected — IDs are an internal identifier,
  not part of the user model. Letting the client choose invites
  collisions and confused tooling (CLI uses `NewUniqueID` too).

### D2. Error code 404 for missing column on create (deliberate departure from move)

`POST /api/cards/:id/move` (V2) returns **400 `COLUMN_NOT_FOUND`** for
an unknown destination. The create handler returns **404
`COLUMN_NOT_FOUND`** for the same condition. Reasoning:

- On move, the path identifies an existing resource (the card)
  and the body proposes a destination — an unknown column is a
  *validation* failure on the client-supplied payload.
- On create, the column the body names is the *resource the
  request is creating into* — semantically more like an unknown
  parent. `404 Not Found` is the standard REST response when the
  referenced parent doesn't exist.

We considered keeping V2's 400 across both for consistency.
Rejected because the client surfaces these differently: the UI's
composer never sends an unknown column (the dropdown is the column
list, which is the same list the server validates), so the practical
difference is invisible to the in-app user. Where the 4xx surfaces
to a CLI/script user (curl, AI agent), 404 is a clearer signal that
the column itself is the problem, not the request shape.

The wire code (`COLUMN_NOT_FOUND`) is unchanged — only the status
code differs. Existing `httpError` mapping returns 400 for
`*ColumnNotFoundError`; the create handler emits its own response
via `writeErrorJSON` rather than going through `httpError`, so the
status divergence is purely in this one new code path.

**Alternatives considered:**

- Match V2 exactly (400): rejected for the resource-not-found
  semantics above.
- Promote V2 to 404 as part of this phase: rejected — out of
  scope, and a behaviour change to an existing endpoint deserves
  its own change.

### D3. `DeleteCard` helper, no `NewCardForColumn` helper

After weighing the optional `NewCardForColumn` helper, this design
keeps the validation chain **inline in the HTTP handler** and only
adds `DeleteCard` at the board layer:

- `DeleteCard` removes an indexed slice element + returns
  `*CardNotFoundError` on miss. It is symmetric with `MoveCard` /
  `UpdateCard` and reusable by future CLI helpers without a wrapper.
- A `NewCardForColumn` helper would duplicate the validation order
  expressed by the handler without any second consumer (the CLI's
  `commands.AddCard` already exists and uses its own primitives).
  Extracting it gains testability symmetry at the cost of a second
  place to track changes whenever priorities / tags rules evolve.

The handler's create code stays compact (~50 LOC) and the new
board-layer surface is just one new symbol (`DeleteCard`).

If a future phase grows a second call site for create (e.g. an MCP
tool that wants to bypass HTTP), promoting the inline validation
into a `NewCardForColumn` helper is a mechanical refactor and a
strict superset of this design.

**Alternatives considered:**

- Add both `DeleteCard` and `NewCardForColumn`: rejected — the
  helper has only one call site and the handler logic is short
  enough to live inline without obscuring the validation chain.
- Push delete into the handler too (no board helper): rejected —
  `DeleteCard` is a clean primitive other tooling (CLI scripts,
  test fixtures) will want, and it mirrors the existing `MoveCard`
  / `UpdateCard` shape.

### D4. Composer state machine — four states, parent-column scope

Composer state lives on the column's Alpine sub-component (ADR 0003
§D10), keyed by the column name. The state machine has four
positions:

```
        openComposer()                 submitComposer()
idle ────────────────────▶ composing ───────────────────▶ submitting
  ▲                          │  │                              │
  │ cancelComposer()         │  │ error from server            │
  │ (Esc / blur /            │  │                              │
  │  empty submit)           │  ▼                              │
  └──────────────────────────┴─ error ◀──────────────────────┘
                                 │
                                 │ user types or hits Add again
                                 ▼
                              composing
```

State variables per column scope:

```js
// inside the column's x-data block
{
  composing: false,
  draft: '',
  error: '',
  submitting: false,
}
```

Transitions:

- `openComposer()` — `composing = true; draft = ''; error = ''`.
- `cancelComposer(force=false)` — return to idle. Triggered by:
  - clicking the Cancel button,
  - pressing Escape,
  - blurring the textarea **only if** `event.relatedTarget`
    is outside the composer (i.e. focus is leaving the whole
    composer surface, not just moving from textarea to Add
    button),
  - submitting with `draft.trim() === ''` (empty submit
    silently cancels).
  When `force === true` (Escape, Cancel button), state resets
  unconditionally including a non-empty draft. When `force === false`
  (blur path), state still resets — there is no draft preservation in
  v1. The flag is reserved for a future "warn before discarding"
  behaviour and currently does nothing extra.
- `submitComposer()` — read `draft.trim()`:
  - empty → call `cancelComposer(true)` and return.
  - non-empty → set `submitting = true`, POST, then:
    - on 2xx → reset to idle (`composing = false`, `submitting = false`,
      `draft = ''`, `error = ''`). SSE delivers the refetch.
    - on non-2xx → set `error = response.error.message || 'HTTP <status>'`,
      `submitting = false`, stay in composing (textarea + Add
      visible, error line visible).
    - on fetch reject → same as non-2xx with `e.message`.

Keyboard handling on the textarea:

- `Enter` (no modifier) → `submitComposer()`.
- `Shift+Enter` → default behaviour (insert newline).
- `Escape` → `cancelComposer(true)`.

The Add button is `type="submit"` inside the composer's
`<form @submit.prevent="submitComposer()">` so keyboard /
screen-reader users get standard form semantics. The Cancel button
is `type="button"` to avoid a form-submission round-trip.

### D5. Hover delete button — Sortable-aware click guard

The `<button class="card-delete">` lives inside each `.card`,
absolutely positioned top-right. CSS hides it (`opacity: 0;
pointer-events: none;`) by default and reveals it on
`.card:hover`. The button itself has `:hover` styles that tint
background + glyph to `--danger-*`.

Two click hazards exist and both are handled in `deleteCard(id, evt)`:

1. **Modal trigger conflict.** The `.card` element carries
   `@click="openCard(card)"` from V3. Clicking the × must NOT
   bubble up and open the modal. Handler starts with
   `evt.stopPropagation()`. (`.prevent` is not needed because
   the click does not have a default action that matters.)

2. **Drag-end mouseup conflict.** Sortable.js fires
   `pointerdown` / `pointermove` / `pointerup` on the card body.
   When a user drags a card and releases the mouse over the ×
   region, the `click` event that follows can naively trigger
   `deleteCard`. The guard:
   - `board()` already calls `handleDrop(evt)` from
     Sortable's `onEnd`. We extend the Alpine root with a
     transient flag `_dragJustEnded` set to `true` inside
     `onEnd` and cleared via `setTimeout(() => this._dragJustEnded = false, 0)`.
   - `deleteCard` checks `_dragJustEnded` and returns early
     if it is true (the post-drop `click` always fires in the
     same macrotask as `onEnd`, so a 0 ms timeout is sufficient
     to skip the immediate-following click and re-arm for normal
     clicks). This is the same "drag cleanup pattern" already
     used by libraries like `react-dnd` and demoed in Sortable's
     own examples — a known idiom, not a novel trick.

`deleteCard` algorithm:

```js
async deleteCard(id, evt) {
  if (evt) evt.stopPropagation();
  if (this._dragJustEnded) return;
  try {
    const res = await fetch('/api/cards/' + encodeURIComponent(id), {
      method: 'DELETE',
    });
    if (!res.ok) {
      // 404 → card already gone (CLI raced us). Force a refetch so
      // the UI catches up. Other errors: log and refetch anyway.
      console.warn('delete failed', res.status);
      await this.load();
      return;
    }
    // Success: SSE will refetch. No optimistic mutation here.
  } catch (e) {
    console.error('delete request errored, refetching', e);
    await this.load();
  }
}
```

There is no client-side confirmation. The user-visible affordance is
two layers of friction — hover the card to reveal the ×, then hover
the × itself to see the danger tint, then click. Misclicks on the
card body still open the V3 modal (no destructive default).

### D6. Wire shape — JSON envelope mirrors V2/V3

POST request body:

```json
{
  "column":      "todo",
  "title":       "Finish UI-4 spec",
  "description": "",
  "priority":    "",
  "tags":        []
}
```

`description`, `priority`, and `tags` are optional. When absent
they default to the empty string / empty slice. Snake_case per
ADR 0002 §D7.

POST success response (201):

```json
{
  "card": {
    "id":          "a4f9c2",
    "title":       "Finish UI-4 spec",
    "column":      "todo",
    "priority":    "",
    "tags":        [],
    "description": "",
    "created_at":  "2026-05-21T16:42:09Z",
    "updated_at":  "2026-05-21T16:42:09Z"
  }
}
```

`cardToResponse` (existing helper) handles the encoding so the
created card uses the same shape as `/api/board` cards.

DELETE response (200):

```json
{"deleted": "a4f9c2"}
```

200-with-body chosen over 204 No Content so the client can use a
single `await res.json()` codepath for every endpoint. The extra
bytes are negligible.

Error envelope on both endpoints follows ADR 0001 §D8:

```json
{"error": {"code": "MISSING_TITLE", "message": "...", "details": {...}}}
```

Existing `writeErrorJSON` is reused. No new error codes minted
(ADR 0003 §D9 explicitly enumerates the reuse-only constraint for
UI-4).

### D7. Validation order — match `UpdateCard`'s pre-mutation chain

The create handler runs:

1. Decode JSON → `INVALID_BODY` (400) on malformed input.
2. `strings.TrimSpace(title) == ""` → `MISSING_TITLE` (400).
3. For each tag in `tags`, `strings.TrimSpace(tag) == ""` →
   `INVALID_TAG` (400) with `details.tag` = the offending raw value.
4. Column not in `b.Board.Columns` → `COLUMN_NOT_FOUND` (404 — see D2).
5. `priority != "" && !slices.Contains(b.Board.Priorities, priority)` →
   `INVALID_PRIORITY` (400) with `details.priority`.
6. Generate ID via `board.NewUniqueID(existingIDs)` → on
   `ErrIDExhausted`, fall through `httpError` → 500 `IO_ERROR`.
7. Build the `board.Card` value:
   - `ID` from step 6.
   - `Title` = the trimmed-but-otherwise-verbatim input (we keep
     internal whitespace; only leading / trailing are removed).
   - `Column` = the requested column.
   - `Description`, `Priority`, `Tags` from the body (defaulted).
   - `CreatedAt = UpdatedAt = time.Now().UTC().Truncate(time.Second)`.
8. `board.AppendCardToColumn(b, card)`.
9. `board.Save(s.boardPath, b)` → on error, `httpError` (500).
10. Respond `201 Created` with `{"card": cardToResponse(card)}`.

Order matches `UpdateCard` so a future `NewCardForColumn` refactor
can extract it verbatim. Title is validated before tags because
the title is the only required field; reporting `INVALID_TAG` when
the user actually forgot to type a title would be confusing.

The delete handler is shorter:

1. Read `id` from path.
2. `board.Load(s.boardPath)` → `httpError` on failure.
3. `board.DeleteCard(b, id)` → `*CardNotFoundError` → 404
   `CARD_NOT_FOUND` via existing `httpError` mapping.
4. `board.Save(s.boardPath, b)` → `httpError` on failure.
5. Respond `200 OK` with `{"deleted": "<id>"}`.

On failure paths 2–4, the on-disk file is byte-unchanged: `Load`
errors before any mutation, `DeleteCard` errors before the slice
splice when the ID misses, and `Save` is atomic so a marshal /
rename error rolls back via the temp-file cleanup (`os.Remove` in
`Save`).

### D8. UI composer markup — Alpine sub-scope on `<div class="column-footer">`

The composer is a sub-`x-data` block attached to the column
footer, **not** new global state on `board()`. ADR 0003 §D10
mandates this shape: each column already has a per-column scope
implicit in the `<template x-for>` over `columns`; the footer's
`x-data` adds its private composer state without polluting the
root component.

```html
<template x-for="col in columns" :key="col">
  <section class="column">
    <header class="list-header">
      <h2 x-text="col"></h2>
      <span class="list-count mono-counter" x-text="cards_per_column[col] || 0"></span>
    </header>
    <ul class="cards" :data-column="col">
      <template x-for="card in cardsByColumn(col)" :key="card.id">
        <li class="card" :data-card-id="card.id" @click="openCard(card)">
          <!-- card body unchanged from V3 -->
          <button type="button"
                  class="card-delete"
                  aria-label="Delete card"
                  @click="deleteCard(card.id, $event)">×</button>
        </li>
      </template>
    </ul>
    <div class="column-footer"
         x-data="{ composing: false, draft: '', error: '', submitting: false,
                   openComposer() { this.composing = true; this.draft = ''; this.error = ''; },
                   cancelComposer() { this.composing = false; this.draft = ''; this.error = ''; },
                   async submitComposer() {
                     const title = (this.draft || '').trim();
                     if (!title) { this.cancelComposer(); return; }
                     this.submitting = true; this.error = '';
                     try {
                       const res = await fetch('/api/cards', {
                         method: 'POST',
                         headers: {'Content-Type': 'application/json'},
                         body: JSON.stringify({ column: col, title })
                       });
                       if (!res.ok) {
                         const err = await res.json().catch(() => ({}));
                         this.error = (err && err.error && err.error.message) || ('HTTP ' + res.status);
                         this.submitting = false;
                         return;
                       }
                       this.composing = false; this.draft = ''; this.submitting = false;
                       // SSE handles the refetch; no manual reload here.
                     } catch (e) {
                       this.error = e.message || String(e);
                       this.submitting = false;
                     }
                   }
                 }">
      <button x-show="!composing"
              type="button"
              class="button-ghost composer-open"
              @click="openComposer()">+ Add a card</button>
      <form x-show="composing"
            class="composer"
            @submit.prevent="submitComposer()">
        <textarea x-model="draft"
                  x-ref="composerInput"
                  x-init="$watch('composing', v => v && $nextTick(() => $refs.composerInput.focus()))"
                  rows="2"
                  placeholder="Enter a title…"
                  @keydown.enter.prevent="submitComposer()"
                  @keydown.shift.enter="/* allow newline */ true"
                  @keydown.escape="cancelComposer()"
                  @blur="if (!$event.relatedTarget || !$event.relatedTarget.closest('.composer')) cancelComposer()"
                  :disabled="submitting"></textarea>
        <div class="composer-actions">
          <button type="submit"
                  class="button-primary"
                  :disabled="submitting"
                  x-text="submitting ? 'Adding…' : 'Add'"></button>
          <button type="button"
                  class="button-ghost"
                  @click="cancelComposer()">Cancel</button>
        </div>
        <p class="composer-error" x-show="error" x-text="error"></p>
      </form>
    </div>
  </section>
</template>
```

Two notes on the snippet:

- `col` is captured by the outer `template x-for`; the
  inline-arrow `submitComposer` reads it via the closure Alpine
  creates over the `for` iteration variable. Alpine 3 supports
  this idiom — used in V3's tag-chip removal handler too.
- The Shift+Enter handler is a no-op (`true`) only to opt out of
  Alpine's default `.enter` swallow when Shift is held. The
  textarea's native newline behaviour fires after the Alpine
  handler resolves.

### D9. CSS — hover delete + composer surface, all tokenised

```css
/* Card hover-delete affordance (per refs/design.md §Card) */
.card { position: relative; }
.card-delete {
  position: absolute;
  top: var(--space-sm);
  right: var(--space-sm);
  width: 22px; height: 22px;
  border-radius: 11px;
  border: 0;
  background: var(--surface);
  color: var(--text-muted);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  opacity: 0;
  pointer-events: none;
  transition: opacity 120ms ease, background 120ms ease, color 120ms ease;
}
.card:hover .card-delete {
  opacity: 1;
  pointer-events: auto;
}
.card-delete:hover {
  background: var(--danger-bg);
  color: var(--danger-fg);
}

/* Column-foot composer (per refs/design.md §Composers + §List footer) */
.column-footer { padding: var(--space-sm); }
.composer-open {
  width: 100%;
  justify-content: flex-start;
  color: var(--text-muted);
}
.composer {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--rounded-lg);
  padding: var(--space-md);
}
.composer textarea {
  resize: vertical;
  min-height: 56px;
  background: transparent;
  border: 0;
  outline: none;
  font: inherit;
  color: var(--text);
}
.composer:focus-within {
  border-color: var(--accent);
}
.composer-actions { display: flex; gap: var(--space-sm); }
.composer-error { color: var(--danger-fg); margin: 0; font-size: 12px; }
```

The danger tokens (`--danger-bg`, `--danger-fg`) and the focus
accent (`--accent`) are minted in UI-1's token sweep. No new
variables are introduced.

### D10. SSE refetch reliance — server-of-truth contract

Per ADR 0002 §D9, the server fires a `board-changed` SSE event
whenever `kanban.toml` is rewritten — including the viewer's own
writes. The successful POST / DELETE response on the client does
**not** mutate `this.cards` directly. The SSE listener
(`handleExternalChange`) already calls `this.load()`, which
re-fetches `/api/board` and replaces the local arrays.

This keeps the local model exactly one source-of-truth refresh
away from disk. If the SSE pipeline is down (e.g. `EventSource`
errored), the client still gets a response body it could apply
optimistically — the spec leaves that as a future enhancement and
the v1 behaviour is "client sees the change on the next reconnect
+ refetch", which `EventSource` does automatically at 2 s
intervals per the server's `retry: 2000` directive.

The composer's local state resets to idle on the response — that
reset is purely cosmetic (close the composer surface, clear the
draft) and does not touch the board's card list. There is no path
where local card state diverges from disk by more than the SSE
round-trip.

### D11. No support for creating into a hidden filter view

UI-3's filter is purely cosmetic (display: none on non-matching
cards). When the user submits a composer while a filter is active,
the new card might not match the filter and therefore be
immediately hidden after the SSE refetch. The composer does not
clear the filter on submit — that would be a behaviour change to
UI-3. The user-visible result is "I added a card and it didn't
appear", but the badge count on the filter button updates if the
card matches, and the column's `cards_per_column` count increments
unconditionally (per ADR 0003 §D8 — column counts are board
counts, not filter counts).

This is documented here, not specced, because it's a no-op for
the v1 user experience: the typical filter use is "find a card by
substring", not "narrow the create surface".

## Risks / Trade-offs

- **Sortable click-after-drag race.** The `_dragJustEnded` flag
  handles the common case (mouseup over the ×). A pathological
  case — drag the card, hold for longer than one tick, release
  over the × — could theoretically slip through. Mitigation: the
  delete is idempotent at the server (404 on a re-delete is
  harmless) and the SSE refetch immediately reconciles. If real
  users report ghosted deletes, raise the timeout from 0 to 50 ms.

- **No drag of the composer surface.** The composer is a `<form>`
  inside the column footer, outside the `.cards <ul>` Sortable
  targets, so it is not draggable. This is intentional: composers
  are per-column and the affordance reads naturally as part of the
  column.

- **Form-of-truth ambiguity during submission.** Between the POST
  response and the SSE refetch (typically < 500 ms), the local
  `this.cards` does not yet contain the new card. The composer
  has already closed. A user who immediately re-opens the composer
  sees a clean state. The visual lag is identical to V3's modal
  save path and has not been a usability complaint there.

- **Empty-board first-card UX.** Opening a brand-new board, every
  column shows the V1 `.empty` placeholder. The composer footer
  still appears below the placeholder. This is the intended
  Redacto pattern (`refs/design.md` §"List footer") — the
  placeholder is the empty state, the composer is the action.

- **Title-only creation surface.** The composer accepts only a
  title. Users who want to set priority / tags on creation must
  create then click the card to open the modal. This is a
  deliberate friction reduction in the common case; advanced
  fields can still be set via the CLI or the HTTP endpoint (which
  accepts all fields).

- **Delete vs SSE order.** If the network is slow, the client may
  see the SSE `board-changed` (fired by the server *after* its
  internal `Save`) before the DELETE response resolves. In that
  case `handleExternalChange` triggers a refetch that already
  reflects the deletion; the subsequent DELETE response is
  effectively a no-op for the client. No visible glitch — the
  card disappears once, when the refetch arrives.

## Migration Plan

Not applicable. Additive change. Existing CLI behaviour for
`ezida add` / `ezida rm` is unchanged. The new HTTP endpoints are
net-new; no client cutover work.

## Open Questions

None within this phase. The optional `NewCardForColumn` helper
decision is locked in D3 (skip it).

## References

- ADR 0001 §D7 (envelope), §D8 (error code namespace).
- ADR 0002 §D3 (server-of-truth), §D7 (snake_case wire), §D8
  (partial-update semantics), §D9 (SSE + refetch).
- ADR 0003 §D2 (stack), §D9 (reuse codes), §D10 (composer Alpine
  sub-component), §D13 (no delete confirm).
- `refs/design.md` §"Card", §"Composers", §"List footer".
- `internal/board/board.go` (existing `AppendCardToColumn`,
  `MoveCard`).
- `internal/board/id.go` (`NewUniqueID`, `ErrIDExhausted`).
- `internal/board/update.go` (validation-error templates).
- `internal/server/handlers.go` (route registration, `httpError`
  mapping).
- Archive: `2026-05-21-add-card-inline-edit/design.md` (modal +
  validation chain reference shape).
