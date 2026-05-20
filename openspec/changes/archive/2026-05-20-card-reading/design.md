## Context

P1 shipped `internal/board` as a pure library. P2 builds the CLI on top
of it: the binary entry point, four read commands (`init`, `board`,
`list`, `get`), the shared output layer (text + JSON + color + errors +
exit codes), and the AND-combined filter engine.

All cross-cutting choices — JSON shape, error envelope, exit codes,
color rules, init overwrite policy — are settled in
`openspec/decisions/0001-kanban-v1-batch.md`. This design focuses on the
P2-specific architecture: how the cobra tree is laid out, how renderers
are organized, and how typed `board` errors map to CLI exit codes and
JSON error codes.

## Goals / Non-Goals

**Goals:**
- Land a runnable `ezida` binary with read-only functionality.
- Pin the JSON contract for `board`, `list`, `get` so the skill and
  later phases can rely on it without revisiting.
- Establish a shared output layer that P3 / P4 reuse unchanged for
  write commands.
- Cover every command and every filter combination with table-driven
  tests against fixtures.

**Non-Goals:**
- No mutating commands (`add`, `edit`, `move`, `rm`) — P3.
- No board-configuration commands (`columns`, `priorities`) — P4.
- No skill embedding (`ezida init` does NOT write `SKILL.md` yet) — P5.
- No release artifact, no install script — P6.

## Decisions

### File layout

```
cmd/ezida/
  main.go              # cobra root, --json / --no-color / --version, dispatch
internal/commands/
  init.go              # `ezida init`
  board.go             # `ezida board`
  list.go              # `ezida list`
  get.go               # `ezida get`
  commands_test.go     # end-to-end command tests against fixtures
internal/output/
  text.go              # table renderer, key:value renderer, color helpers
  json.go              # JSON envelopes for board, list, get, errors
  exit.go              # ExitOK / ExitUserError / ExitSystemError + Fail()
  output_test.go       # renderer unit tests
```

### Cobra root and command registration

The root command in `cmd/ezida/main.go` registers the four read
commands and exposes two persistent flags:

```go
var jsonOut bool
var noColor bool

var rootCmd = &cobra.Command{
    Use:     "ezida",
    Short:   "File-based Kanban for software projects",
    Version: version, // injected by ldflags in P6
}

func init() {
    rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "emit JSON to stdout")
    rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
    rootCmd.AddCommand(commands.NewInitCmd(&jsonOut))
    rootCmd.AddCommand(commands.NewBoardCmd(&jsonOut))
    rootCmd.AddCommand(commands.NewListCmd(&jsonOut))
    rootCmd.AddCommand(commands.NewGetCmd(&jsonOut))
}

func main() {
    output.ConfigureColor(noColor)
    if err := rootCmd.Execute(); err != nil {
        output.Fail(err, jsonOut)
    }
}
```

Each command constructor returns `*cobra.Command` whose `RunE` calls
`board.Load(boardPath)`, applies the command logic, hands the result to
the output layer, and returns any error verbatim. The root catches the
error and routes it through `output.Fail`.

The board file path is hard-coded to `./kanban.toml` for v1 (per brief
§5 — single file at project root). No `--file` flag; revisit if multi-
board support lands post-v1.

### Output layer

`internal/output` is the only package that touches stdout/stderr. It
holds:

- `ConfigureColor(force bool)`: inspects stdout's `Stat()` for
  `ModeCharDevice`, checks `os.Getenv("NO_COLOR")`, applies the
  `--no-color` override. Stores the resolved decision in a package
  variable consulted by `Color()` / `Plain()` helpers.
- `Table(rows [][]string, headers []string) string`: computes per-column
  widths, joins with two-space separators, returns the rendered string.
- `KeyValue(pairs []KV) string`: renders aligned `Key:   Value` lines.
- `JSONBoard`, `JSONList`, `JSONGet`, `JSONError`: pure functions
  returning `[]byte` (compact, single-line, newline-terminated).
- `Fail(err error, asJSON bool)`: switch on typed errors from `board`
  to derive the error code and exit code:
  - `*board.SchemaVersionError` → code `SCHEMA_VERSION_MISMATCH`, exit 1.
  - `*board.ValidationError` → code `VALIDATION_FAILED`, exit 1.
  - `errors.Is(err, fs.ErrNotExist)` against `kanban.toml` → code
    `BOARD_NOT_FOUND`, exit 1, message suggests `ezida init`.
  - `errors.Is(err, fs.ErrPermission)` → code `IO_ERROR`, exit 2.
  - `*commands.CardNotFoundError` → `CARD_NOT_FOUND`, exit 1.
  - `*commands.InvalidFilterError` → `INVALID_FILTER`, exit 1.
  - `*commands.AlreadyInitializedError` → `ALREADY_INITIALIZED`, exit 1.
  - default → code `IO_ERROR`, exit 2.

### Filter engine for `list`

`list` builds a pipeline of `func(c board.Card) bool` predicates from
the flag values:

```go
type filter func(board.Card) bool

func buildFilters(column, titleContains, tag, priority string, b *board.Board) ([]filter, error) {
    var fs []filter
    if column != "" {
        if !slices.Contains(b.Board.Columns, column) {
            return nil, &InvalidFilterError{Flag: "column", Value: column}
        }
        fs = append(fs, func(c board.Card) bool { return c.Column == column })
    }
    if titleContains != "" {
        needle := strings.ToLower(titleContains)
        fs = append(fs, func(c board.Card) bool {
            return strings.Contains(strings.ToLower(c.Title), needle)
        })
    }
    if tag != "" {
        fs = append(fs, func(c board.Card) bool { return slices.Contains(c.Tags, tag) })
    }
    if priority != "" {
        if !slices.Contains(b.Board.Priorities, priority) {
            return nil, &InvalidFilterError{Flag: "priority", Value: priority}
        }
        fs = append(fs, func(c board.Card) bool { return c.Priority == priority })
    }
    return fs, nil
}
```

Filters are AND-combined by iterating `b.Cards` and keeping cards for
which every predicate returns `true`. Validating unknown column /
priority values up-front (not silently returning zero results) is the
better UX and matches the spec's `INVALID_FILTER` scenario.

`--tag` does NOT validate the tag against any list — tags are
free-form. If the tag does not match any card, the result is the empty
set, which is success (exit 0).

### Init implementation

```go
func runInit(cmd *cobra.Command, args []string) error {
    if _, err := os.Stat(boardPath); err == nil && !force {
        return &AlreadyInitializedError{Path: boardPath}
    }
    b := &board.Board{
        SchemaVersion: board.SupportedSchemaVersion,
        Board: board.BoardConfig{
            Columns:    parseCSVOrDefault(columns, defaultColumns),
            Priorities: parseCSVOrDefault(priorities, defaultPriorities),
        },
    }
    if err := board.Save(boardPath, b); err != nil {
        return err
    }
    if !jsonOut {
        fmt.Fprintln(cmd.OutOrStdout(), "initialized kanban.toml")
    } else {
        fmt.Fprintln(cmd.OutOrStdout(), `{"initialized":true,"path":"kanban.toml"}`)
    }
    return nil
}
```

`parseCSVOrDefault` trims whitespace around each value and rejects
empty entries. Duplicates within `--columns` or `--priorities` are
caught by `board.Validate` (called inside `board.Save`) — no custom
check needed here.

### Testing approach

`internal/commands/commands_test.go` uses cobra's built-in
`SetArgs` + `SetOut` / `SetErr` capture to run each command against
in-memory fixtures (real files under `testdata/` so the atomic-rename
path is exercised). Each test table row asserts:

- Exit code (or returned error type via `errors.As`).
- Stdout content (exact for JSON, normalized whitespace for text).
- Stderr content (substring match for human messages).

`output_test.go` covers the renderers in isolation: table alignment
with empty cells, JSON envelope shape, color-off behavior under
`NO_COLOR`.

## Risks / Trade-offs

- **Hard-coded `kanban.toml` path** → users with the file in a
  subdirectory must `cd` first. Trade-off: matches brief §5 ("at the
  project root"), removes a flag from every command, simpler skill
  documentation. Revisit if user feedback contradicts.
- **Filter validation cost** → tag filter scans `b.Cards` once per
  predicate. For 10k-card boards this is microseconds; acceptable.
- **Cobra dependency size** → ~1 MB compiled. Acceptable given the
  ergonomic and stability gain over a stdlib `flag`-based parser.
- **No `--file` / `--path` override** → blocks running `ezida` over a
  shared file from a sibling directory. Could be added without breaking
  the JSON contract later if real demand surfaces.
- **JSON contract is now load-bearing** → every subsequent phase MUST
  treat changes to these envelopes as breaking. P3 / P4 / P5 add fields
  but never rename or remove.

## Migration Plan

Not applicable — `ezida` did not exist before P1, and P1 is a library
with no users. The first end users of P2 are the developer running the
binary and the skill in P5.

## Open Questions

None.
