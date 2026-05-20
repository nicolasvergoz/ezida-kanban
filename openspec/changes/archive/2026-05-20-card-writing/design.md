## Context

P2 made the binary readable; P3 makes it mutating. The three commands
(`add`, `move`, `rm`) share the same skeleton: load board → validate
inputs → mutate in memory → re-validate → save atomically → render
result. The only command with new ergonomic concerns is `rm`, which
needs interactive confirmation while also supporting non-interactive
script use.

Cross-cutting choices (atomic write, error envelope, exit codes, JSON
shape) are settled in `openspec/decisions/0001-kanban-v1-batch.md`. This
design covers the P3-specific bits: command file layout, the
mutating-command skeleton, the TTY detection helper, and the new error
types.

## Goals / Non-Goals

**Goals:**
- Land `add`, `move`, `rm` with text and JSON modes.
- Add typed errors for the new failure cases and wire them into the
  shared `output.Fail` dispatcher introduced in P2.
- Keep the interactive prompt for `rm` deterministically testable
  (capturable via `os.Pipe`).
- Preserve the invariant that no `kanban.toml` is ever written without
  passing `board.Validate`.

**Non-Goals:**
- No `edit` command — P4.
- No column / priority management — P4.
- No card reordering within a column — out of scope for v1 (brief §11).
- No undo / history — out of scope for v1.

## Decisions

### File layout (additions)

```
internal/commands/
  add.go              # NewAddCmd, runAdd
  move.go             # NewMoveCmd, runMove
  rm.go               # NewRmCmd, runRm, promptConfirm
  errors.go           # +ColumnNotFoundError, InvalidPriorityError,
                      #  MissingTitleError, InvalidTagError,
                      #  InteractiveRequiredError
  mutate.go           # shared helper: applyMutation(b, mutateFn) error
  commands_test.go    # +TestAdd_*, TestMove_*, TestRm_* tables
internal/output/
  exit.go             # +cases for the 5 new typed errors
internal/tty/
  tty.go              # isTTY(*os.File) bool — stdlib-only TTY detection
```

`internal/tty` is its own package so command tests can stub it (the
test binary's stdin/stdout are not TTYs; the prompt logic needs a
behavior-equivalent escape).

### Mutating-command skeleton

`internal/commands/mutate.go` exposes:

```go
func mutateAndSave(mutate func(b *board.Board) (board.Card, error)) (board.Card, error) {
    b, err := board.Load(boardPath)
    if err != nil { return board.Card{}, err }
    c, err := mutate(b)
    if err != nil { return board.Card{}, err }
    if err := board.Save(boardPath, b); err != nil {
        return board.Card{}, err
    }
    return c, nil
}
```

Each command's `RunE` consists of: flag parsing → call `mutateAndSave`
with a closure that performs the in-memory mutation → render the
returned card (or rm-specific envelope). `board.Save` already runs
`board.Validate` per the P1 contract, so the re-validation invariant is
inherited.

### `add` specifics

```go
func runAdd(args []string, column, priority, descr, tagsCSV string, jsonOut bool) error {
    if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
        return &MissingTitleError{}
    }
    tags, err := parseTags(tagsCSV)
    if err != nil { return err }

    card, err := mutateAndSave(func(b *board.Board) (board.Card, error) {
        if !slices.Contains(b.Board.Columns, column) {
            return board.Card{}, &ColumnNotFoundError{Name: column}
        }
        if priority != "" && !slices.Contains(b.Board.Priorities, priority) {
            return board.Card{}, &InvalidPriorityError{Name: priority}
        }
        existing := make([]string, 0, len(b.Cards))
        for _, c := range b.Cards { existing = append(existing, c.ID) }
        id, err := board.NewUniqueID(existing)
        if err != nil { return board.Card{}, err }
        now := time.Now().UTC().Truncate(time.Second)
        c := board.Card{
            ID: id, Title: args[0], Column: column,
            Description: descr, Tags: tags, Priority: priority,
            CreatedAt: now, UpdatedAt: now,
        }
        board.AppendCardToColumn(b, c)
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

`parseTags` splits on `,`, trims whitespace, and returns
`*InvalidTagError` if any resulting element is empty. Tags are stored
in declaration order with duplicates preserved (brief §5 imposes no
uniqueness constraint on tags; document this as a deliberate non-rule
if user demand arises).

### `move` specifics

```go
func runMove(idArg, columnArg string, jsonOut bool) error {
    card, err := mutateAndSave(func(b *board.Board) (board.Card, error) {
        if !slices.Contains(b.Board.Columns, columnArg) {
            return board.Card{}, &ColumnNotFoundError{Name: columnArg}
        }
        idx := indexCardByID(b.Cards, idArg)
        if idx < 0 { return board.Card{}, &CardNotFoundError{ID: idArg} }

        c := b.Cards[idx]
        b.Cards = slices.Delete(b.Cards, idx, idx+1)
        c.Column = columnArg
        c.UpdatedAt = time.Now().UTC().Truncate(time.Second)
        board.AppendCardToColumn(b, c)
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

Same-column moves still go through the delete-then-append path. The
card lands at the end of the column section — this is a deliberate
no-op visually only when the card was already the last in its column.
Brief §7.6 prescribes "bottom of column" placement on every move;
honor it uniformly to keep the rule simple.

### `rm` specifics — interactive confirmation

```go
type rmFlags struct{ yes bool }

func runRm(idArg string, flags rmFlags, jsonOut bool) error {
    if jsonOut && !flags.yes {
        return &InteractiveRequiredError{Hint: "use --yes with --json"}
    }
    b, err := board.Load(boardPath)
    if err != nil { return err }
    idx := indexCardByID(b.Cards, idArg)
    if idx < 0 { return &CardNotFoundError{ID: idArg} }
    title := b.Cards[idx].Title

    if !flags.yes {
        if !tty.IsTTY(os.Stdin) || !tty.IsTTY(os.Stdout) {
            return &InteractiveRequiredError{Hint: "use --yes for non-interactive"}
        }
        ok, err := promptConfirm(os.Stderr, os.Stdin, fmt.Sprintf(
            `Delete card %s %q? [y/N] `, idArg, title))
        if err != nil { return err }
        if !ok {
            fmt.Fprintln(os.Stderr, "aborted")
            return nil
        }
    }

    b.Cards = slices.Delete(b.Cards, idx, idx+1)
    if err := board.Save(boardPath, b); err != nil { return err }

    if jsonOut {
        fmt.Printf("{\"id\":%q,\"deleted\":true}\n", idArg)
    } else {
        fmt.Printf("removed %s\n", idArg)
    }
    return nil
}

func promptConfirm(w io.Writer, r io.Reader, msg string) (bool, error) {
    fmt.Fprint(w, msg)
    reader := bufio.NewReader(r)
    line, err := reader.ReadString('\n')
    if err != nil && err != io.EOF { return false, err }
    answer := strings.TrimSpace(strings.ToLower(line))
    return answer == "y", nil
}
```

`promptConfirm` takes the writer and reader as parameters so the test
suite can drive it with `bytes.Buffer` and `strings.Reader`. The
production call wires `os.Stderr` (so the prompt does not pollute
stdout pipes) and `os.Stdin`.

### TTY detection

```go
// package tty

func IsTTY(f *os.File) bool {
    info, err := f.Stat()
    if err != nil { return false }
    return (info.Mode() & os.ModeCharDevice) != 0
}
```

Stdlib-only per ADR §D1. Works on macOS/Linux (sufficient for v1
targets). Tests stub by passing a real `os.File` from `os.CreateTemp`
(returns `IsTTY = false`).

### Error type additions

```go
type ColumnNotFoundError   struct{ Name string }
type InvalidPriorityError  struct{ Name string }
type MissingTitleError     struct{}
type InvalidTagError       struct{ Raw string }
type InteractiveRequiredError struct{ Hint string }
```

Each implements `Error() string` with a focused human sentence.
`output.Fail` maps them to the codes `COLUMN_NOT_FOUND`,
`INVALID_PRIORITY`, `MISSING_TITLE`, `INVALID_TAG`,
`INTERACTIVE_REQUIRED`, all exit code 1.

### JSON helpers

P2's `output.JSONList` / `JSONGet` already cover the read shapes. P3
adds:

```go
func JSONCard(c board.Card) []byte // returns {"card":{...}}\n
```

`JSONCard` includes the `description` field (mutating commands echo
the full card, unlike `list`). `rm`'s success envelope is generated
inline with `fmt.Printf` because it is a one-off shape that does not
warrant a helper.

## Risks / Trade-offs

- **Move always re-orders** → trades a tiny extra write (re-emitting
  the cards slice) for a single, predictable rule. Acceptable.
- **`--yes` required with `--json`** → forces script authors to be
  explicit. Worth it: a silent prompt in JSON mode would hang
  scripts indefinitely.
- **No `--dry-run`** → a useful flag to add post-v1; out of scope here
  to keep P3 narrow.
- **`updated_at` refresh on no-op moves** → debatable but consistent
  with "any modification command refreshes the timestamp". Documented
  in the spec scenario.
- **Tag duplicates allowed** → matches brief silence on the topic. If
  user demand surfaces, a later phase can de-dupe in `parseTags`.

## Migration Plan

Not applicable. P3 adds capability; no existing data shape changes.

## Open Questions

None.
