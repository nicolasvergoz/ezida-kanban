## 1. Binary bootstrap

- [x] 1.1 Create `cmd/ezida/main.go` with a minimal `func main()` that prints `"ezida"` and exits 0. Done when `go run ./cmd/ezida` exits 0 and prints `ezida`.
- [x] 1.2 Add `github.com/spf13/cobra` to `go.mod` via `go get`. Done when `go.mod` lists `cobra` and `go build ./...` exits 0.
- [x] 1.3 Replace `main.go` content with a cobra root command (`Use: "ezida"`) carrying persistent `--json` and `--no-color` flags and a `--version` placeholder (`version = "dev"`). Done when `ezida --help` lists the flags and `ezida --version` prints `ezida version dev`.

## 2. Output layer

- [x] 2.1 Create `internal/output/exit.go` with `ExitOK = 0`, `ExitUserError = 1`, `ExitSystemError = 2` constants and a `Fail(err error, asJSON bool)` function that switches on the typed errors listed in the design (returns `SCHEMA_VERSION_MISMATCH`, `VALIDATION_FAILED`, `BOARD_NOT_FOUND`, `IO_ERROR`). Done when a unit test maps each error type to the expected code/exit pair.
- [x] 2.2 Create `internal/output/text.go` with `ConfigureColor(force bool)`, `Table(rows [][]string, headers []string) string`, `KeyValue(pairs []KV) string`. Color detection uses `os.Stdout.Stat()` and `os.Getenv("NO_COLOR")`. Done when `output_test.go` confirms (a) `NO_COLOR=1` suppresses ANSI, (b) non-TTY suppresses ANSI, (c) table widths align across rows with empty cells.
- [x] 2.3 Create `internal/output/json.go` with `Board`, `List`, `Get`, `Error` envelope helpers returning `[]byte`. Each helper marshals with `encoding/json` and appends `\n`. Done when round-trip tests parse the helpers' output and assert the expected keys.
- [x] 2.4 Add typed CLI errors `CardNotFoundError`, `InvalidFilterError`, `AlreadyInitializedError` in `internal/commands/errors.go`. Each implements `Error() string` and is recognised by `output.Fail`. Done when `output_test.go` covers each via `errors.As`.

## 3. `ezida init`

- [x] 3.1 Create `internal/commands/init.go` exposing `NewInitCmd(jsonOut *bool) *cobra.Command`. The command parses `--columns`, `--priorities`, `--force`. Default columns = `["todo","ongoing","done"]`, default priorities = `["low","medium","high"]`. Done when `ezida init --help` lists the flags.
- [x] 3.2 Implement the run logic: refuse with `AlreadyInitializedError` if `kanban.toml` exists and `--force` is not set; otherwise build a `*board.Board` and call `board.Save`. Done when both scenarios in the spec ("Fresh init with defaults" and "Init refuses to overwrite") pass.
- [x] 3.3 Add success outputs: text mode prints `initialized kanban.toml`, JSON mode prints `{"initialized":true,"path":"kanban.toml"}`. Done when the corresponding tests in `commands_test.go` pass.
- [x] 3.4 Add `TestInit_*` table-driven tests covering: defaults, custom columns, custom priorities, refuses without force, succeeds with force, custom values with duplicates (validation error surfaces). Done when `go test ./internal/commands -run TestInit` exits 0.

## 4. `ezida board`

- [x] 4.1 Create `internal/commands/board.go` with `NewBoardCmd(jsonOut *bool)`. Done when `ezida board --help` runs.
- [x] 4.2 Implement the run logic: `board.Load("kanban.toml")`, compute `cards_per_column` by iterating `b.Cards`, render text or JSON per spec. Done when both spec scenarios for `ezida board` pass against a fixture.
- [x] 4.3 Add `TestBoard_TextOutput`, `TestBoard_JSONOutput`, `TestBoard_MissingFile` (asserting `BOARD_NOT_FOUND`). Done when all three pass.

## 5. `ezida list`

- [x] 5.1 Create `internal/commands/list.go` with `NewListCmd(jsonOut *bool)` and flags `--column`, `--title-contains`, `--tag`, `--priority`. Done when `ezida list --help` lists the four filters.
- [x] 5.2 Implement `buildFilters` per design (validates `--column` and `--priority` against the loaded board, returns `*InvalidFilterError` on unknown values). Done when `TestList_InvalidColumnFilter` and `TestList_InvalidPriorityFilter` both return `INVALID_FILTER` in JSON mode.
- [x] 5.3 Implement the run logic: apply filters AND-combined, render text (aligned table with header) or JSON (`{"cards":[...]}` with `description` omitted). Done when `TestList_DescriptionOmittedInJSON` confirms no `description` key appears in any card.
- [x] 5.4 Add table-driven tests for filter combinations: none, single column, single tag, column+tag, column+priority, case-insensitive title, no match returns empty list with exit 0. Done when `go test ./internal/commands -run TestList` exits 0.

## 6. `ezida get`

- [x] 6.1 Create `internal/commands/get.go` with `NewGetCmd(jsonOut *bool)`. The command takes exactly one positional arg (the card ID). Done when `ezida get` (no arg) exits 1 with a usage error and `ezida get a3f2k9 --help` runs.
- [x] 6.2 Implement the run logic: lookup card by exact ID, return `*CardNotFoundError` on miss, render text (key:value block) or JSON (`{"card":{...}}` with `description` included). Done when both spec scenarios for `get` pass.
- [x] 6.3 Add `TestGet_FoundText`, `TestGet_FoundJSON`, `TestGet_NotFound`, `TestGet_PriorityOmittedInJSON`. Done when all pass.

## 7. Wiring and version stamp

- [x] 7.1 Register the four command constructors on the cobra root in `cmd/ezida/main.go`. Done when `ezida --help` lists `init`, `board`, `list`, `get`.
- [x] 7.2 Replace the literal `"dev"` version with a `version` package-level variable that can be overridden by `-ldflags "-X main.version=v0.1.0"`. Done when `go build -ldflags "-X main.version=v0.1.0" ./cmd/ezida && ./ezida --version` prints `ezida version v0.1.0`.
- [x] 7.3 Add a top-level `kanban.toml` fixture under `internal/commands/testdata/populated.toml` (3 todo, 1 ongoing, 7 done, mixed priorities, varied tags) used by every command test. Done when `ls internal/commands/testdata/` lists `populated.toml`.

## 8. Acceptance gate

- [x] 8.1 Run `go test ./... && go vet ./...` from the repo root. Done when both exit 0.
- [x] 8.2 Manually verify the JSON envelopes against the spec's example shapes by running `ezida board --json | jq .`, `ezida list --json | jq '.cards | length'`, `ezida get <id> --json | jq .card.description` on the fixture. Done when each command's output matches the spec's example structure.
- [x] 8.3 Verify color behavior: `ezida list` in a terminal shows aligned plain output (no ANSI sequences for v1; reserved for future); `ezida list | cat` shows no ANSI. Done when both observations hold.
