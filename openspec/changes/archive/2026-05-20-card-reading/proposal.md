## Why

P1 produced the `board-storage` library but no executable. P2 turns
`ezida` into a working read-only CLI: a `kanban.toml` can be created with
`ezida init`, inspected with `ezida board`, listed with `ezida list`, and
drilled into with `ezida get`. This is the first phase that delivers user
value and the first phase that pins the JSON contract every later phase
and the embedded skill will rely on.

## What Changes

- Add `cmd/ezida/main.go`: the binary entry point and the `cobra` root
  command. Register a global persistent `--json` flag, a global
  `--no-color` flag, and the standard `--help` / `--version` handling.
- Add `internal/commands/` with one file per command:
  - `init.go`: writes a new `kanban.toml` with defaults or with values
    from `--columns` and `--priorities`. Refuses to overwrite an existing
    file unless `--force` is passed (ADR §D15). Does NOT yet write the
    embedded skill — that lands in P5.
  - `board.go`: prints the schema version, columns (with per-column card
    counts), and priorities. Text format per ADR §D9; JSON shape per
    ADR §D7.
  - `list.go`: prints all cards, with `--column`, `--title-contains`,
    `--tag`, `--priority` filters AND-combined. Text format = aligned
    table with header; JSON omits the `description` field per ADR §D7.
  - `get.go`: prints one card's full detail by ID. Text = key:value
    block; JSON = full card object including `description`.
- Add `internal/output/` with a small render layer:
  - `text.go`: aligned table renderer, key:value renderer, color helpers
    (auto-detect TTY, respect `NO_COLOR`, honor `--no-color`).
  - `json.go`: stable JSON envelopes for board, list, get, and the error
    shape from ADR §D8.
  - `exit.go`: exit-code helper (`ExitOK = 0`, `ExitUserError = 1`,
    `ExitSystemError = 2`) and a `Fail(err)` helper that classifies typed
    `board` errors into the right code and renders them to stderr.
- Add tests covering every command's text and JSON output against
  fixtures, plus filter combinations for `list`.

## Capabilities

### New Capabilities
- `card-reading`: the `init`, `board`, `list`, `get` commands, their
  filters, their text format, their JSON contract, and the shared
  output layer (color, exit codes, error envelope).

### Modified Capabilities
None.

## Impact

- New code: `cmd/ezida/`, `internal/commands/`, `internal/output/`.
- New dependency: `github.com/spf13/cobra` (per ADR §D1).
- No new runtime dependencies for end users — the binary stays
  self-contained.
- Pins the JSON contract for downstream phases and for the skill.
  Subsequent phases extend it (add the `card` envelope returned by
  write commands) but MUST NOT break the shapes defined here.
