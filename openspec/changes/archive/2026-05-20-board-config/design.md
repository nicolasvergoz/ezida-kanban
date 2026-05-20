## Context

P4 finishes the CLI surface for v1. The two non-trivial pieces are:

1. **Propagation** — renaming a column or priority must update every
   referencing card in the same write. Simple in practice because the
   whole board is rewritten on each `Save`, but the implementation must
   not skip validation.
2. **Refusal-with-detail** — `rm` on an in-use column or priority must
   produce an error whose payload lists every offending card with `id`
   and `title`. This requires shaping the error output (both text and
   JSON) in a richer way than the previous flat messages.

The columns and priorities sub-trees share 95% of their logic. The
design factors the common parts into a single helper so the two
sub-commands stay thin.

Cross-cutting choices (atomic write, error envelope, exit codes) come
from `openspec/decisions/0001-kanban-v1-batch.md`. This design covers
P4-specific architecture.

## Goals / Non-Goals

**Goals:**
- Land `edit`, `columns add|rename|rm`, `priorities add|rename|rm`.
- Provide a shared `refgroup` helper so columns and priorities share
  the same add / rename / rm logic with parameterized
  "what does referencing mean" predicates.
- Surface the refusal-with-detail payload uniformly in text and JSON
  modes by enriching the `output.Fail` dispatcher.
- Keep the file invariant (`board.Validate` clean after every save).

**Non-Goals:**
- No card reordering within a column — out of scope for v1.
- No `--position` for priorities — re-init is the v1 escape hatch.
- No `--dry-run` — same reason as in P3.

## Decisions

### File layout (additions)

```
internal/commands/
  edit.go             # NewEditCmd, runEdit
  columns.go          # NewColumnsCmd → add/rename/rm subcommands
  priorities.go       # NewPrioritiesCmd → add/rename/rm subcommands
  refgroup.go         # parameterized helpers shared by columns + priorities
  errors.go           # +ColumnInUseError, PriorityInUseError,
                      #  DuplicateError, PositionOutOfRangeError,
                      #  LastColumnError, LastPriorityError, NothingToEditError
internal/output/
  exit.go             # +cases for the 7 new typed errors; richer
                      #  text/JSON rendering for "refusal-with-detail"
```

### Shared `refgroup` helper

Columns and priorities share three operations and three error patterns,
parameterized only by:

- The list to mutate (`&b.Board.Columns` vs `&b.Board.Priorities`).
- The card-field accessor (`func(c *board.Card) *string`) — `&c.Column`
  vs `&c.Priority`.
- The "is referenced" predicate (a card with empty `Priority` is not
  considered referencing — important so `priorities rm` ignores cards
  without a priority).
- The error-code labels (`COLUMN_IN_USE` / `LAST_COLUMN` /
  `COLUMN_NOT_FOUND` vs the priority equivalents).

```go
type refGroup struct {
    list           *[]string
    cardField      func(*board.Card) *string
    isReferencing  func(value, name string) bool  // see below
    inUseErr       func(name string, cards []affectedCard) error
    lastErr        func() error
    duplicateErr   func(name string) error
    unknownErr     func(name string) error
    positionErr    func() error
}

func (g *refGroup) add(name string, position int) error { /* ... */ }
func (g *refGroup) rename(old, new string, cards []board.Card) error { /* ... */ }
func (g *refGroup) remove(name string, cards []board.Card) error { /* ... */ }
```

`isReferencing(value, name)` for columns is `value == name`. For
priorities it is `value != "" && value == name` — cards without a
priority MUST NOT count as referencing.

Each command's `RunE` builds a `refGroup` for its target list, then
calls one of the three methods inside `mutateAndSave`. The columns
and priorities files end up ~30 lines each, mostly wiring.

`refgroup.add` validates: name not duplicate, position in range
`[1, len+1]`, then `slices.Insert`. `refgroup.rename` validates: old
exists, new not duplicate, then mutates both the list entry and every
referencing card. `refgroup.remove` validates: name exists, no
references (collect violators first), not last-of-list, then
`slices.Delete`.

### `edit` implementation

```go
type editFlags struct {
    title       *string
    description *string
    priority    *string
    tags        *string
    column      *string
}

func runEdit(idArg string, f editFlags, jsonOut bool) error {
    if f.title == nil && f.description == nil && f.priority == nil &&
       f.tags == nil && f.column == nil {
        return &NothingToEditError{}
    }
    card, err := mutateAndSave(func(b *board.Board) (board.Card, error) {
        idx := indexCardByID(b.Cards, idArg)
        if idx < 0 { return board.Card{}, &CardNotFoundError{ID: idArg} }

        c := b.Cards[idx]
        if f.title != nil {
            if strings.TrimSpace(*f.title) == "" {
                return board.Card{}, &MissingTitleError{}
            }
            c.Title = *f.title
        }
        if f.description != nil { c.Description = *f.description }
        if f.priority != nil {
            if *f.priority != "" && !slices.Contains(b.Board.Priorities, *f.priority) {
                return board.Card{}, &InvalidPriorityError{Name: *f.priority}
            }
            c.Priority = *f.priority
        }
        if f.tags != nil {
            tags, err := parseTags(*f.tags)
            if err != nil { return board.Card{}, err }
            c.Tags = tags
        }
        c.UpdatedAt = time.Now().UTC().Truncate(time.Second)

        if f.column != nil {
            if !slices.Contains(b.Board.Columns, *f.column) {
                return board.Card{}, &ColumnNotFoundError{Name: *f.column}
            }
            c.Column = *f.column
            b.Cards = slices.Delete(b.Cards, idx, idx+1)
            board.AppendCardToColumn(b, c)
        } else {
            b.Cards[idx] = c
        }
        return c, nil
    })
    if err != nil { return err }

    if jsonOut {
        os.Stdout.Write(output.JSONCard(card))
    } else {
        fmt.Println(card.ID)
    }
    return nil
}
```

The pointer types (`*string` instead of `string`) let cobra distinguish
"flag not passed" from "flag passed empty". Cobra exposes
`cmd.Flags().Changed("name")` for this; the helper wraps it into the
pointer struct at the top of `RunE` to keep `runEdit` clean.

### Refusal-with-detail payload

`internal/commands/errors.go` adds:

```go
type affectedCard struct {
    ID    string `json:"id"`
    Title string `json:"title"`
}

type ColumnInUseError struct {
    Name  string
    Cards []affectedCard
}

func (e *ColumnInUseError) Error() string {
    var sb strings.Builder
    fmt.Fprintf(&sb, "column %q still referenced by %d cards:\n", e.Name, len(e.Cards))
    for _, c := range e.Cards {
        fmt.Fprintf(&sb, "  %s  %s\n", c.ID, c.Title)
    }
    sb.WriteString("Move or remove these cards first.")
    return sb.String()
}
```

`PriorityInUseError` mirrors this exactly.

`output.Fail` recognises these types and:

- In text mode: prefixes the first line with `Error: ` and emits the
  rest of `Error()` verbatim (already shaped).
- In JSON mode: extracts the structured `Cards` slice into
  `error.details.cards` and uses a short `message` like
  `"column \"todo\" still referenced by 2 cards"`.

To keep `output.Fail` simple, the typed errors expose a small interface:

```go
type detailedError interface {
    error
    Code() string
    Details() any
    ShortMessage() string
}
```

All seven new P4 errors implement this; existing P2/P3 errors gain a
default implementation via `defaultError{code, message}` so the
dispatcher branches only on `errors.As(detailedError)`.

### Cobra sub-tree

```
ezida
├── init / board / list / get  (P2)
├── add / move / rm            (P3)
├── edit                       (P4)
├── columns
│   ├── add    [--position=N]
│   ├── rename <old> <new>
│   └── rm     <name>
└── priorities
    ├── add
    ├── rename <old> <new>
    └── rm     <name>
```

`NewColumnsCmd` and `NewPrioritiesCmd` register the three sub-commands
each. The sub-commands' help text is concise; usage examples live in
README (P6).

## Risks / Trade-offs

- **Pointer-vs-string flag pattern** → cobra has no first-class "flag
  was passed" predicate; using `Flags().Changed` adds boilerplate. The
  encapsulation in `editFlags` confines that boilerplate to one place.
- **`refgroup` indirection** → adds one layer of abstraction for the
  sake of removing ~150 lines of near-duplicate logic. Trade-off is
  worth it; the helper is small (under 100 lines) and isolates the
  columns-vs-priorities differences to the constructor.
- **`detailedError` interface** → introduces a typed contract for
  errors. Acceptable: the P3 errors are wrapped trivially. The interface
  stays internal to the binary; the JSON shape is the public contract.
- **`--position` semantics asymmetry** → columns expose it, priorities
  do not. Documented in the spec. Justification: priorities have a
  natural low→high ordering, which makes mid-list insertion conceptually
  fraught (where does "urgent" go among `low/medium/high`?).

## Migration Plan

Not applicable.

## Open Questions

None.
