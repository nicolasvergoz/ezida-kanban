## Context

V1 wired the read-only path: `GET /api/board` + a renderer.
`board.AppendCardToColumn` is the only mutation primitive in
`internal/board` and is used by the CLI's `add`/`move` commands.
ADR 0002 §D4 deferred the introduction of `InsertCardAt` until a
second caller needed a positional insert; that moment has arrived
with the drag-and-drop interaction.

The HTTP layer has no positional move endpoint yet. This change
adds it, layered on `MoveCard` which sits on `InsertCardAt`. The
UI wires `Sortable.js` to issue the request after each drop.

## Goals / Non-Goals

**Goals:**
- A clean primitive (`InsertCardAt`) reusable by V3 (if needed) and
  any future move-style operation.
- Drag-and-drop across columns and within a column.
- Optimistic UI: the card visually moves on drop, the server call
  fires after. On failure, refetch and re-render so the UI never
  drifts from disk.
- Position clamping per ADR 0002 §D11.

**Non-Goals:**
- No undo, no toast for the success case.
- No CLI-side positional move (`ezida move <id> <col>` still appends).
- No multi-select drag, no copy on drop, no cross-document drag.
- No reorder of columns themselves (a column-management UI is V3+
  territory).

## Decisions

### `InsertCardAt` semantics

```go
// InsertCardAt inserts c into b.Cards so that, after the insert,
// the card occupies the given 0-indexed position among cards whose
// Column equals column. position is clamped to
// [0, len(cardsInColumn)] (where cardsInColumn excludes any
// existing entry with the same ID — relevant when called from
// MoveCard mid-relocation). Sets c.Column = column.
func InsertCardAt(b *Board, c Card, column string, position int)
```

Implementation sketch:

```go
func InsertCardAt(b *Board, c Card, column string, position int) {
    c.Column = column
    // Build the list of flat indices currently occupied by `column`,
    // excluding any matching c.ID (caller may pass an existing card).
    var colIdx []int
    for i, x := range b.Cards {
        if x.Column == column && x.ID != c.ID {
            colIdx = append(colIdx, i)
        }
    }
    // Clamp position.
    if position < 0 { position = 0 }
    if position > len(colIdx) { position = len(colIdx) }
    // Compute insertion point in the flat slice.
    var insertAt int
    switch {
    case len(colIdx) == 0:
        insertAt = len(b.Cards) // first card of an empty column → append
    case position == len(colIdx):
        insertAt = colIdx[len(colIdx)-1] + 1
    default:
        insertAt = colIdx[position]
    }
    b.Cards = append(b.Cards, Card{})
    copy(b.Cards[insertAt+1:], b.Cards[insertAt:len(b.Cards)-1])
    b.Cards[insertAt] = c
}
```

### `MoveCard` semantics

```go
func MoveCard(b *Board, id, column string, position int) error {
    // Find current index of id.
    var curIdx = -1
    for i, c := range b.Cards {
        if c.ID == id { curIdx = i; break }
    }
    if curIdx < 0 { return &CardNotFoundError{ID: id} }
    // Validate column.
    if !containsString(b.Board.Columns, column) {
        return &ColumnNotFoundError{Column: column}
    }
    // Pull the card out.
    c := b.Cards[curIdx]
    b.Cards = append(b.Cards[:curIdx], b.Cards[curIdx+1:]...)
    // Refresh timestamp and re-insert.
    c.UpdatedAt = time.Now().UTC().Truncate(time.Second)
    InsertCardAt(b, c, column, position)
    return nil
}
```

### `AppendCardToColumn` refactor

```go
func AppendCardToColumn(b *Board, c Card) {
    count := 0
    for _, x := range b.Cards {
        if x.Column == c.Column { count++ }
    }
    InsertCardAt(b, c, c.Column, count)
}
```

External behavior unchanged. Existing tests for this helper must
keep passing untouched.

### `POST /api/cards/:id/move` handler

```go
type movePayload struct {
    Column   string `json:"column"`
    Position int    `json:"position"`
}

func (s *server) handleMove(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id") // or stdlib pathvalue equivalent
    var p movePayload
    if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
        s.writeError(w, &InvalidBodyError{Reason: err.Error()})
        return
    }
    b, err := board.Load(s.boardPath)
    if err != nil { s.writeError(w, err); return }
    if err := board.MoveCard(b, id, p.Column, p.Position); err != nil {
        s.writeError(w, err)
        return
    }
    if err := board.Save(s.boardPath, b); err != nil {
        s.writeError(w, err)
        return
    }
    // Re-find the card to return its post-move state.
    for _, c := range b.Cards {
        if c.ID == id {
            json.NewEncoder(w).Encode(map[string]any{"card": c})
            return
        }
    }
}
```

Path routing: V1's handler bag is a plain `http.ServeMux`. Go 1.22+
`ServeMux` supports `r.PathValue("id")` for patterns like
`POST /api/cards/{id}/move`. No router dependency added.

### Sortable.js wiring (client)

```js
// inside board()
mountSortable() {
  document.querySelectorAll('.cards').forEach((ul) => {
    Sortable.create(ul, {
      group: 'cards',
      animation: 0,                // animations land in polish phase
      ghostClass: 'sortable-ghost',
      onEnd: async (evt) => {
        const id = evt.item.dataset.cardId;
        const column = evt.to.dataset.column;
        const position = evt.newIndex;
        try {
          const res = await fetch(`/api/cards/${id}/move`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ column, position }),
          });
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
        } catch (e) {
          console.error('move failed, refetching', e);
          await this.load();
        }
      },
    });
  });
},
```

Each `.cards` `<ul>` gets `data-column="<name>"`. Each `<li.card>`
gets `data-card-id="<id>"`. `mountSortable()` is called once Alpine
finishes the initial render and again after every `load()`.

### Idempotence + self-write

The server's atomic `Save` triggers the V4 file watcher, but V4
isn't in this phase. In V2 the broadcast doesn't exist — clients
won't refetch on their own moves. Optimistic UI is the only
post-move feedback. Acceptable: the only divergence path is a
failure, which we handle by full refetch.

## Risks / Trade-offs

- **Move during board reload**: if the user drags while a hypothetical
  refresh is mid-fetch, Sortable's drop may report an index against
  a stale render. Mitigation: refetch on any error. The clamping in
  `InsertCardAt` absorbs minor index mismatches silently.
- **Concurrent clients**: two browsers dragging at the same time
  race on `Save`. Last writer wins per ADR 0002 §D3. The watcher
  in V4 will repair divergence once it ships.
- **No animation**: Sortable can animate the rearrangement but
  `animation: 0` keeps it minimal per the design constraint.
- **Position clamping hides client bugs**: if Sortable ever reports a
  negative index, the clamp silently fixes it. We log the raw value
  in the server's request log? — V1 has no logging. Accept the
  silent fix in v1.

## Migration Plan

Not applicable. Backwards compatible: existing CLI behavior
preserved, `AppendCardToColumn` semantics preserved.

## Open Questions

- Should `MoveCard` refuse a move to the card's current column at the
  same position (no-op)? Decision: no-op succeeds and refreshes
  `updated_at` (consistent with CLI `move` no-op per existing
  `card-writing` spec). The test list captures the scenario.
