## Context

V3 ([archived `add-card-inline-edit`](../archive/2026-05-21-add-card-inline-edit/))
shipped the modal as an "always-open form": title input, description
textarea, priority `<select>`, tag chip editor, Save + Cancel buttons.
A single `saveCard()` packs all four fields into one PATCH; Cancel
discards the whole draft. The pattern works but reads as a form, not
as a card-detail view, which contradicts the Redacto editorial
direction adopted in the UI redesign batch.

The cross-cutting decisions for this batch are pinned in
[ADR 0003](../../decisions/0003-ui-redesign-batch.md):

- §D3: the modal stays for card detail (an explicit override of
  `refs/design.md`'s "no modals anywhere" rule); inside the modal,
  fields use Trello-style click-to-edit.
- §D10: in-modal interactive state lives on the parent component's
  Alpine `x-data` scope — no extracted partials, no Alpine register
  plugin.
- §D2 / §D5: stack frozen (Alpine + vanilla CSS, no build step), every
  visual treatment reads from the token system landed in UI-1.

The server-side `PATCH /api/cards/:id` semantics are pinned in
[ADR 0002 §D8](../../decisions/0002-viewer-batch.md): present key
replaces, absent key untouched, empty string / empty list clears. This
phase reuses that contract verbatim — no server change.

Visual truth for the field treatments (rendered text appearance,
hover affordance, editor styling) is `refs/design.md`; the JSX shell
under `refs/kanban-design/` is consulted only when `design.md` is
ambiguous on a detail
([ADR §D1](../../decisions/0003-ui-redesign-batch.md)).

## Goals / Non-Goals

**Goals:**

- Replace the V3 always-open form with a click-to-edit detail view
  where each editable field has its own rendered/edit/saving/error
  state.
- Commit each field independently via `PATCH /api/cards/:id` with a
  single key per request; on success the server response is the source
  of truth.
- Show server errors inline directly under the field that failed,
  keeping the editor open so the user can fix and retry.
- Preserve modal-level affordances (Esc to close when no field is
  active, click-overlay-to-close) and the V4 close-on-external-change
  behavior.
- Keep the tag chip pattern (chips + add input) but commit per add and
  per remove rather than batching with a Save click.
- Zero server endpoints added or modified — reuse V3's PATCH wire.

**Non-Goals:**

- No global Save / Cancel buttons — every field autosaves.
- No optimistic update — the modal waits for the server response
  before swapping back to rendered mode (matches V3 behavior; keeps
  the failure case simple).
- No focus trap, no aria-live announcements — same a11y scope as V3.
  Revisit when real users hit a wall.
- No multi-field "stage and commit later" mode. If a future workflow
  needs that, it lives in a separate phase.
- No description markdown rendering — plain text only (carried from
  V3 / ADR 0002).
- No undo for a committed edit — disk is the source of truth; the
  CLI / file system already provides version control.
- No new endpoints, no new error codes.
- No change to UI-4 (inline composer at column foot, hover delete) —
  this phase reshapes the modal only.
- No change to the column header / list rename — that's UI-6.

## Decisions

### MD1. Field state machine — `rendered` | `editing` | `saving` | `error`

Each editable field carries its own state machine. Conceptually:

```
            click
rendered ────────────► editing
   ▲                     │
   │  success            │ blur / Enter / comma
   │                     ▼
   │                  saving
   │                  ┌── 2xx ──► (rendered with new value, server response)
   └── Esc on field ──┴── non-2xx ► error (editor stays, message under field)
```

`error` is a sub-state of `editing` from the user's perspective: the
inline editor is preserved with its in-flight value, and a message
renders beneath. Fixing the value and re-committing transitions back
through `saving`.

Implementation: `editing` is a map keyed by field name
(`title`, `description`, `priority`, `tags`); `saving` and `errors`
are sibling maps. Pseudocode:

```js
editing: { title: false, description: false, priority: false, tags: false },
saving:  { title: false, description: false, priority: false, tags: false },
errors:  { title: '',    description: '',    priority: '',    tags: ''    },
drafts:  { title: '',    description: '',    priority: '',    tags: [] },
tagInput: '',
```

`drafts.<name>` holds the in-flight value while a field is in
`editing`. On `Esc`, the draft is discarded and `editing.<name>` flips
back to false without sending a PATCH.

### MD2. One field at a time

Only one field may be in `editing` at any moment. Clicking a different
field while one is editing first commits (blurs) the active one. This
keeps the mental model simple:

- The UI never has multiple unsaved drafts simultaneously — no
  ambiguity about "which one did I edit last?"
- The keyboard `Esc` semantics stay unambiguous: there is at most one
  active field to revert.
- The error display has at most one location at a time.

Alternative considered: allow multiple fields editing simultaneously
(like a spreadsheet). Rejected — adds state-machine surface
(per-field independent timelines, race between commits) for a UX win
that's not warranted at the card-detail granularity. The modal is
small; switching fields with a click is cheap.

### MD3. Per-field commit = single-key PATCH

Each commit issues `PATCH /api/cards/:id` with exactly one key in the
body, e.g.:

```http
PATCH /api/cards/abc123
Content-Type: application/json

{"title": "Refactor authentication"}
```

This exercises the V3-pinned absent-key-untouched semantics (ADR 0002
§D8) cleanly: the server only sees the field that actually changed.
The CLI and other clients can still send multi-key patches — that's
why the wire contract is broader than the UI's exercise of it.

Tags are a special case: the "value" is the full resulting array, not
a per-tag delta. So `addTag('x')` sends
`{"tags": ["existing", "x"]}` and `removeTag('x')` sends
`{"tags": ["existing"]}`. This matches the V3 wire shape (tags is a
replace, not a patch-within-patch), so no server change is needed.

### MD4. Save handler — `saveField(name, value)`

```js
async saveField(name, value) {
  this.errors[name] = '';
  this.saving[name] = true;
  try {
    const res = await fetch(`/api/cards/${this.openId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ [name]: value }),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      this.errors[name] = (err && err.error && err.error.message)
        || `HTTP ${res.status}`;
      return;  // stay in editing
    }
    const body = await res.json();
    this.openCardData = body.card;        // server response = source of truth
    this.editing[name] = false;
    this.drafts[name] = '';                // or [] for tags
  } catch (e) {
    this.errors[name] = e.message;
  } finally {
    this.saving[name] = false;
  }
},
```

The `editing` flag flips back to false only on success; on error it
stays true so the user can fix the value and retry without losing
their typed content. The server response replaces the local
`openCardData` (instead of refetching the whole board) — this is
cheap and keeps the modal's read-only metadata
(`column`, `created_at`, `updated_at`) consistent with the value
just saved.

After every successful commit the page also calls `this.load()` to
refetch `/api/board` so the card lane reflects the new value, exactly
as V3 did. (Cost is one extra round trip per commit; for a single-user
local-only viewer this is fine. If it ever shows up as a problem, the
fix is to patch the in-memory board with the response card; out of
scope here.)

### MD5. Keyboard semantics — Esc is context-sensitive

```
Esc pressed while a field is in `editing`:
  → revert that field, no PATCH

Esc pressed with no field in `editing`:
  → close the modal
```

Implementation: a single `@keydown.escape.window` handler on the modal
overlay checks `Object.values(this.editing).some(Boolean)`. If any
field is editing, it picks the active one and reverts (`drafts[name]`
discarded, `editing[name] = false`). Otherwise it closes the modal
(equivalent to V3's `closeCard()`).

Enter / comma semantics inside specific editors:

- Title input — `Enter` commits the title (no newline allowed in
  titles anyway).
- Description textarea — `Cmd/Ctrl+Enter` commits; plain `Enter`
  inserts a newline (V3 behavior).
- Priority `<select>` — `change` event commits; blur commits.
- Tag-add input — `Enter` or `,` commits the add via `addTag()`
  (which itself PATCHes via `saveField('tags', this.openCardData.tags.concat([newTag]))`).

### MD6. Inline error display

Each field has a dedicated error slot directly beneath the editor:

```html
<div class="field-row">
  <span x-show="!editing.title" class="field" @click="startEdit('title')"
        x-text="openCardData.title"></span>
  <input x-show="editing.title" class="field--editing"
         :class="{ 'field--saving': saving.title }"
         x-model="drafts.title"
         @blur="saveField('title', drafts.title)"
         @keydown.enter.prevent="saveField('title', drafts.title)"
         @keydown.escape.stop.prevent="revertField('title')">
  <p class="field-error" x-show="errors.title" x-text="errors.title"></p>
</div>
```

The error is field-scoped; it doesn't appear at the modal level (V3's
`.modal-error` block is removed). This is closer to the Redacto
direct-manipulation principle: the failure is attached to the thing
that failed.

The `field--saving` class applies a subtle muted-background / dimmed
treatment via CSS only. No spinner, no animation — same restraint as
the V5-polish exclusion that has held since ADR 0002.

### MD7. Tag chips — per-action commits

```js
async addTag() {
  const t = this.tagInput.trim();
  if (!t) return;
  if (this.openCardData.tags.includes(t)) {
    this.tagInput = '';
    return;  // dedup, no PATCH
  }
  const next = [...this.openCardData.tags, t];
  this.tagInput = '';
  await this.saveField('tags', next);
},
async removeTag(t) {
  const next = this.openCardData.tags.filter(x => x !== t);
  await this.saveField('tags', next);
},
```

Each add or remove issues its own PATCH. The chip list reads from
`openCardData.tags` directly (no separate `draft.tags`); the server
response after each commit replaces `openCardData`, which keeps the
chip list and tag input in sync. The tag input is the only thing that
needs a local state (`tagInput`) because it lives between commits.

### MD8. Why Cancel is gone

V3's Cancel button existed because Save was all-or-nothing: typing in
three fields then hitting Cancel meant "throw away all three drafts".
With per-field autosave, there is no aggregated draft to discard.
Each field's `Esc` reverts only its own in-flight value. The modal's
overlay click and Esc-with-no-active-field close the modal without
any prompt — there's nothing to confirm because nothing is pending.

If the user clicks the overlay while a field is mid-`saving`, the
modal close is queued: the in-flight PATCH still completes (browsers
don't cancel fetch on detach unless the request is explicitly aborted,
which we don't do). On success the disk is updated; the user just
doesn't see the result inside the now-closed modal. Acceptable: V4
hot-reload would surface the change anyway on the next render.

### MD9. External change while modal is open

The V4 behavior is preserved: if SSE delivers `event: board-changed`
while the modal is open, the modal closes without prompting and any
in-flight draft is discarded. The viewer-ui spec already has this
requirement ("Open edit modal closes on external change"); this
phase doesn't touch it.

The only subtlety: if a field is mid-`saving` when an external change
arrives, the PATCH still completes (same reason as above) — the next
`load()` will reflect whichever write wins per ADR 0002 §D3
(last-write-wins).

### MD10. Markup structure

```html
<div class="modal-overlay"
     x-show="open"
     @keydown.escape.window="onEscape()"
     @click.self="closeModal()">
  <div class="modal" role="dialog" aria-modal="true">
    <header class="modal-header">
      <span class="modal-id t-mono-label" x-text="openCardData.id"></span>
    </header>

    <!-- Title field -->
    <div class="field-row field-row--title">
      <span class="field t-list-title"
            x-show="!editing.title"
            @click="startEdit('title')"
            x-text="openCardData.title"></span>
      <input type="text"
             class="field--input"
             :class="{ 'field--saving': saving.title }"
             x-show="editing.title"
             x-model="drafts.title"
             x-ref="titleInput"
             @blur="commitField('title')"
             @keydown.enter.prevent="commitField('title')"
             @keydown.escape.stop.prevent="revertField('title')">
      <p class="field-error" x-show="errors.title" x-text="errors.title"></p>
    </div>

    <!-- Description field -->
    <div class="field-row field-row--description">
      <p class="field field--multiline"
         x-show="!editing.description"
         @click="startEdit('description')"
         x-text="openCardData.description || 'Add a description'"></p>
      <textarea class="field--textarea"
                :class="{ 'field--saving': saving.description }"
                x-show="editing.description"
                x-model="drafts.description"
                rows="6"
                @blur="commitField('description')"
                @keydown.meta.enter.prevent="commitField('description')"
                @keydown.ctrl.enter.prevent="commitField('description')"
                @keydown.escape.stop.prevent="revertField('description')"></textarea>
      <p class="field-error" x-show="errors.description" x-text="errors.description"></p>
    </div>

    <!-- Priority field -->
    <div class="field-row field-row--priority">
      <span class="field"
            x-show="!editing.priority"
            @click="startEdit('priority')"
            x-text="openCardData.priority || 'no priority'"></span>
      <select class="field--select"
              :class="{ 'field--saving': saving.priority }"
              x-show="editing.priority"
              x-model="drafts.priority"
              @change="commitField('priority')"
              @blur="commitField('priority')"
              @keydown.escape.stop.prevent="revertField('priority')">
        <option value="">no priority</option>
        <template x-for="p in priorities" :key="p">
          <option :value="p" x-text="p"></option>
        </template>
      </select>
      <p class="field-error" x-show="errors.priority" x-text="errors.priority"></p>
    </div>

    <!-- Tag chips (always live, no rendered-vs-editing split) -->
    <div class="field-row field-row--tags">
      <ul class="tag-chips" :class="{ 'field--saving': saving.tags }">
        <template x-for="t in openCardData.tags" :key="t">
          <li class="tag">
            <span x-text="t"></span>
            <button type="button" @click="removeTag(t)" aria-label="remove tag">&times;</button>
          </li>
        </template>
      </ul>
      <input type="text" class="tag-add"
             x-model="tagInput"
             @keydown.enter.prevent="addTag()"
             @keydown.comma.prevent="addTag()"
             placeholder="add tag">
      <p class="field-error" x-show="errors.tags" x-text="errors.tags"></p>
    </div>

    <!-- Read-only metadata -->
    <footer class="modal-readonly t-mono-label">
      <span>column: <code x-text="openCardData.column"></code></span>
      <span>created: <code x-text="openCardData.created_at"></code></span>
      <span>updated: <code x-text="openCardData.updated_at"></code></span>
    </footer>
  </div>
</div>
```

Note the absence of `<footer class="modal-footer">` with Save / Cancel
buttons — gone by design (MD8).

The `startEdit(name)` helper sets `editing[name] = true`, copies the
current value into `drafts[name]`, and `$nextTick(() => $refs[ref].focus())`
to focus the swapped-in editor.

`commitField(name)` is a thin wrapper that calls
`saveField(name, this.drafts[name])` — it exists so the markup stays
declarative.

`revertField(name)` clears `drafts[name]` and `errors[name]`, then
`editing[name] = false`.

`onEscape()` checks whether any field is editing; if so, it picks the
active one and calls `revertField`; otherwise it calls `closeModal()`.

### MD11. Tags do not have a rendered-only mode

The tag chips are interactive at all times (each chip has its own ×
button; the add input is always visible). This matches Trello's
behavior and is simpler than gating the chip editor behind a
"click to edit tags" state. The `errors.tags` slot still exists for
server-rejection cases (e.g. `INVALID_TAG` on a malformed value).

If the user wants a "view-only" feel for tags, that comes for free —
they simply don't click the chips or type in the input. The CSS still
applies the standard `.field` hover treatment to the chips container
so it's visually consistent with the click-to-edit fields.

### MD12. CSS sketch (tokens come from UI-1)

```css
.field-row {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
  padding: var(--space-sm) 0;
}

.field {
  padding: var(--space-xs) var(--space-sm);
  border-radius: var(--rounded-sm);
  cursor: text;
  transition: background-color 120ms;
}

.field:hover {
  background: color-mix(in oklab, var(--surface) 85%, var(--accent) 15%);
}

.field--input,
.field--textarea,
.field--select {
  padding: var(--space-xs) var(--space-sm);
  border: 1px solid var(--accent);
  border-radius: var(--rounded-sm);
  background: var(--surface);
  color: var(--text);
}

.field--saving {
  opacity: 0.6;
  pointer-events: none;
}

.field-error {
  color: var(--danger);
  font-size: var(--t-body-md-size);
  margin: 0;
}
```

Every color is read from the token system (UI-1 / ADR §D5). No hex
literals.

## Risks / Trade-offs

- **One PATCH per commit is chatty.** Editing a title, description,
  and priority sequentially now sends 3 PATCHes instead of 1.
  At single-user-localhost scale this is negligible. If a future
  network-backed deployment surfaces latency, a debounce-batch layer
  is straightforward to add (still server-compatible — the PATCH wire
  is unchanged).
- **No optimistic update means a network hiccup feels slower** than
  V3's "send and close" path. Acceptable — V3 also waited on the
  response before closing the modal.
- **Esc semantics are context-sensitive**, which is slightly more
  complex than V3's "Esc always closes". Documented in the spec; the
  payoff is that mid-edit Esc doesn't lose unrelated rendered values.
- **Tags-have-no-rendered-mode** means a user who clicks a `×` by
  accident immediately PATCHes a remove with no undo. This is the
  same risk as V4's hover-delete on cards
  ([ADR §D13](../../decisions/0003-ui-redesign-batch.md)): the design
  trades reversibility for directness, and `kanban.toml` is in git.
- **Concurrent edit + external SSE**: same last-write-wins semantics
  as everywhere else (ADR 0002 §D3). The window between PATCH submit
  and response is small enough that this is a theoretical risk only.
- **Click-vs-drag** on the modal's fields is not a concern — Sortable
  only mounts on the column card lists, not inside the modal. So the
  V3 mis-detection risk doesn't apply here.

## Migration Plan

Not applicable in the spec-driven sense — this is a UI-only
refactor. The V3 archived spec is the baseline; this phase modifies
the relevant requirements in `viewer-ui` via a `## MODIFIED
Requirements` delta. The server-side `PATCH /api/cards/:id` is
unchanged; existing V3 server tests continue to pass with no
modification.

Visual regression: the modal looks dramatically different to a user
upgrading from V3. There's no rollout flag in scope — this lands as a
single phase. The look-and-feel batch ordering in ADR §D14 puts UI-5
late enough that any other look-and-feel preferences have already
landed.

## Open Questions

None within this phase. Open items for later phases:

- Should a long description switch to markdown rendering at rest and
  raw markdown in edit mode? Deferred — not in any current phase
  scope; ADR 0002 fixed plain-text-only for V3 and that holds.
- Should the modal grow a "delete" affordance? UI-4 added
  `DELETE /api/cards/:id` for hover-delete on the card itself; the
  modal does not gain a delete button in v1.
