## Context

V1 renders the board, V2 enables drag/move via Sortable.js +
`POST .../move`. V3 closes the editing loop by mirroring the
CLI's `ezida edit` semantics over HTTP. The patch semantics are
pinned in ADR 0002 §D8: present key replaces, absent key
untouched, empty string/empty list clears.

## Goals / Non-Goals

**Goals:**
- Click a card → modal opens with current values pre-filled.
- Edit title, description, tags, priority and save back to disk.
- Validation errors return inline in the modal without closing it.
- Keyboard shortcuts: `Esc` cancels, `Enter` in title saves,
  `Cmd/Ctrl+Enter` in description saves.
- Tag input as chips: type and press Enter (or comma) to add,
  click a chip's × to remove.
- Priority dropdown lists every value from `[board].priorities`
  plus a "no priority" entry that maps to `""`.

**Non-Goals:**
- No card creation or deletion from the UI (use CLI). Modal has
  no "Delete" button in v1.
- No markdown rendering in description (plain text only — brief
  §7 + ADR 0002 footprint).
- No autosave; explicit Save / Cancel.
- No optimistic update on save — the modal blocks on the response
  before closing.
- No drag from inside the modal.
- No multi-card edit / bulk apply.

## Decisions

### `CardPatch` shape

```go
type CardPatch struct {
    Title       *string   `json:"title,omitempty"`
    Description *string   `json:"description,omitempty"`
    Tags        *[]string `json:"tags,omitempty"`
    Priority    *string   `json:"priority,omitempty"`
}
```

Pointers distinguish "key absent in JSON" (pointer nil) from
"key present with empty value" (pointer non-nil to empty
string/slice). Go's `encoding/json` populates the pointer iff
the key is in the input.

### `UpdateCard` algorithm

```go
func UpdateCard(b *Board, id string, p CardPatch) error {
    var idx = -1
    for i, c := range b.Cards {
        if c.ID == id { idx = i; break }
    }
    if idx < 0 { return &CardNotFoundError{ID: id} }
    c := b.Cards[idx]
    if p.Title != nil {
        if strings.TrimSpace(*p.Title) == "" {
            return &MissingTitleError{}
        }
        c.Title = *p.Title
    }
    if p.Description != nil { c.Description = *p.Description }
    if p.Tags != nil {
        for _, t := range *p.Tags {
            if strings.TrimSpace(t) == "" {
                return &InvalidTagError{Tag: t}
            }
        }
        c.Tags = *p.Tags
    }
    if p.Priority != nil {
        if *p.Priority != "" && !containsString(b.Board.Priorities, *p.Priority) {
            return &InvalidPriorityError{Priority: *p.Priority}
        }
        c.Priority = *p.Priority
    }
    c.UpdatedAt = time.Now().UTC().Truncate(time.Second)
    b.Cards[idx] = c
    if verr := Validate(b); verr != nil { return verr }
    return nil
}
```

`UpdateCard` runs `Validate` after the mutation to catch any
post-condition the helper didn't explicitly enforce (e.g. the
schema invariants from `internal/board/validation.go`).

### `PATCH /api/cards/:id` handler

```go
func (s *server) handlePatch(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    var patch board.CardPatch
    if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
        s.writeError(w, &InvalidBodyError{Reason: err.Error()})
        return
    }
    b, err := board.Load(s.boardPath)
    if err != nil { s.writeError(w, err); return }
    if err := board.UpdateCard(b, id, patch); err != nil {
        s.writeError(w, err)
        return
    }
    if err := board.Save(s.boardPath, b); err != nil {
        s.writeError(w, err)
        return
    }
    for _, c := range b.Cards {
        if c.ID == id {
            json.NewEncoder(w).Encode(map[string]any{"card": c})
            return
        }
    }
}
```

### Modal HTML (Alpine)

```html
<div class="modal-overlay"
     x-show="editing"
     @keydown.escape.window="closeCard()"
     @click.self="closeCard()">
  <div class="modal" role="dialog" aria-modal="true" x-show="editing">
    <header class="modal-header">
      <h2>Edit card</h2>
      <span class="modal-id" x-text="draft.id"></span>
    </header>
    <form @submit.prevent="saveCard()">
      <label>Title
        <input type="text" x-model="draft.title" required
               @keydown.enter.prevent="saveCard()">
      </label>
      <label>Description
        <textarea x-model="draft.description" rows="6"
                  @keydown.meta.enter.prevent="saveCard()"
                  @keydown.ctrl.enter.prevent="saveCard()"></textarea>
      </label>
      <label>Priority
        <select x-model="draft.priority">
          <option value="">no priority</option>
          <template x-for="p in priorities" :key="p">
            <option :value="p" x-text="p"></option>
          </template>
        </select>
      </label>
      <fieldset class="tags-field">
        <legend>Tags</legend>
        <ul class="tag-chips">
          <template x-for="t in draft.tags" :key="t">
            <li class="tag">
              <span x-text="t"></span>
              <button type="button" @click="removeTag(t)" aria-label="remove tag">&times;</button>
            </li>
          </template>
        </ul>
        <input type="text" x-model="tagInput"
               @keydown.enter.prevent="addTag()"
               @keydown.comma.prevent="addTag()"
               placeholder="add tag, press Enter">
      </fieldset>
      <p class="modal-error" x-show="error" x-text="error"></p>
      <div class="modal-readonly">
        <span>column: <code x-text="draft.column"></code></span>
        <span>created: <code x-text="draft.created_at"></code></span>
        <span>updated: <code x-text="draft.updated_at"></code></span>
      </div>
      <footer class="modal-footer">
        <button type="button" @click="closeCard()">Cancel</button>
        <button type="submit">Save</button>
      </footer>
    </form>
  </div>
</div>
```

### Alpine state additions

```js
// inside board()
editing: false,
draft: null,        // shallow copy of the card under edit
tagInput: '',
error: '',
openCard(card) {
  this.draft = { ...card, tags: [...(card.tags || [])] };
  this.tagInput = '';
  this.error = '';
  this.editing = true;
},
closeCard() {
  this.editing = false;
  this.draft = null;
},
addTag() {
  const t = this.tagInput.trim();
  if (!t) return;
  if (!this.draft.tags.includes(t)) this.draft.tags.push(t);
  this.tagInput = '';
},
removeTag(t) {
  this.draft.tags = this.draft.tags.filter(x => x !== t);
},
async saveCard() {
  this.error = '';
  const body = {
    title: this.draft.title,
    description: this.draft.description,
    tags: this.draft.tags,
    priority: this.draft.priority,
  };
  try {
    const res = await fetch(`/api/cards/${this.draft.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      this.error = (err && err.error && err.error.message) || `HTTP ${res.status}`;
      return;
    }
    this.closeCard();
    await this.load();
  } catch (e) {
    this.error = e.message;
  }
},
```

The card click handler is added in the card template: `@click="openCard(card)"`.
Drag interaction (Sortable.js from V2) and click do not conflict — Sortable
fires `onEnd` only after a drag distance threshold; pure clicks pass through.

### Field replacement semantics on the wire

`saveCard()` always sends all four fields, so the patch is effectively
a PUT for those fields. This is intentional: the UI has the full
state, and sending the full set avoids "did the user clear it or
leave it untouched?" ambiguity. The server-side `UpdateCard` still
honors absent-key semantics for other clients (CLI scripts, AI
assistants) that might send partial patches.

## Risks / Trade-offs

- **Click vs drag**: Sortable.js's default drag threshold is small
  but non-zero. Mis-detection in either direction is rare. If users
  report false drags, raise the threshold via Sortable's `delay` or
  `touchStartThreshold` options — out of scope for this phase.
- **Tag chip UX is minimal**: no autocomplete, no fuzzy match,
  no validation against existing tags. Adding the same tag twice
  is silently deduped client-side.
- **Modal accessibility**: role/aria attributes are set but no
  focus trap is implemented. Tab can leave the modal. Acceptable
  for v1; revisit if real users hit it.
- **External change while editing**: V4 will close the modal on
  external change (per ADR 0002 §D9). V3 leaves the modal open
  because SSE isn't wired yet; the next refresh resets state.
- **Description in JSON**: card descriptions can be large. Sending
  the full description on every save is fine; no size limit in v1.

## Migration Plan

Not applicable. Additive change.

## Open Questions

None within this phase.
