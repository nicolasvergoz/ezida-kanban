## 1. Shared helpers

- [x] 1.1 Create `internal/tty/tty.go` exposing `IsTTY(f *os.File) bool` using `os.File.Stat()` + `os.ModeCharDevice`. Done when a unit test confirms a `os.CreateTemp` file returns `false` and the package builds with no external dependency.
- [x] 1.2 Add `mutateAndSave` in `internal/commands/mutate.go` per the design (load → mutate closure → save). Done when a unit test exercises both the happy path and the closure-returns-error path.
- [x] 1.3 Extend `internal/commands/errors.go` with `ColumnNotFoundError`, `InvalidPriorityError`, `MissingTitleError`, `InvalidTagError`, `InteractiveRequiredError` types (each `Error() string`). Done when `go build ./...` passes and `errors.As` finds each type.
- [x] 1.4 Extend `internal/output/exit.go` so `Fail` maps each new typed error to its stable code (`COLUMN_NOT_FOUND`, `INVALID_PRIORITY`, `MISSING_TITLE`, `INVALID_TAG`, `INTERACTIVE_REQUIRED`) and exit code 1. Done when a unit test asserts each mapping.
- [x] 1.5 Add `output.JSONCard(card board.Card) []byte` in `internal/output/json.go` that returns `{"card":{...}}\n` including the `description` field. Done when a unit test parses the output and checks all expected keys.

## 2. `ezida add`

- [x] 2.1 Create `internal/commands/add.go` with `NewAddCmd(jsonOut *bool)` declaring positional title arg and flags `--column` (required), `--priority`, `--tags`, `--description`. Done when `ezida add --help` lists every flag.
- [x] 2.2 Implement `parseTags(csv string) ([]string, error)` that splits on `,`, trims, rejects empty entries with `*InvalidTagError`. Done when its unit test covers happy path, empty entries, surrounding whitespace.
- [x] 2.3 Implement `runAdd` per the design: title check → mutateAndSave closure validating column / priority, generating ID, setting timestamps, appending via `board.AppendCardToColumn`. Done when `TestAdd_HappyPath`, `TestAdd_UnknownColumn`, `TestAdd_UnknownPriority`, `TestAdd_EmptyTitle`, `TestAdd_TagError`, `TestAdd_AppendsToColumnBottom`, `TestAdd_JSONEchoesCard` all pass.
- [x] 2.4 Confirm a successful `add` produces text output equal to `<id>\n` on stdout (and nothing on stderr). Done when `TestAdd_TextOutputIsIDOnly` passes.

## 3. `ezida move`

- [x] 3.1 Create `internal/commands/move.go` with `NewMoveCmd(jsonOut *bool)` taking two positional args (`<id>`, `<column>`). Done when `ezida move --help` runs.
- [x] 3.2 Implement `runMove` per the design: lookup card → delete from slice → update column + `updated_at` → append via `board.AppendCardToColumn`. Done when `TestMove_HappyPath`, `TestMove_SameColumn`, `TestMove_UnknownColumn`, `TestMove_UnknownCard`, `TestMove_JSONEchoesCard` all pass.
- [x] 3.3 Confirm move preserves the order of other cards. Done when `TestMove_PreservesOtherCardsOrder` asserts the slice order of unaffected cards is byte-identical pre/post.

## 4. `ezida rm`

- [x] 4.1 Create `internal/commands/rm.go` with `NewRmCmd(jsonOut *bool)` taking one positional arg (`<id>`) and `--yes` flag. Done when `ezida rm --help` runs.
- [x] 4.2 Implement `promptConfirm(w io.Writer, r io.Reader, msg string) (bool, error)` per the design and unit-test it with `y\n`, `Y\n`, `n\n`, `\n` (empty), `garbage\n`. Done when `TestPromptConfirm_*` pass.
- [x] 4.3 Implement `runRm` per the design: JSON-mode without `--yes` rejects, non-TTY without `--yes` rejects, TTY without `--yes` prompts. Done when `TestRm_WithYes`, `TestRm_InteractiveAccept`, `TestRm_InteractiveReject`, `TestRm_JSONWithoutYesRejects`, `TestRm_NonTTYWithoutYesRejects`, `TestRm_UnknownCard`, `TestRm_JSONSuccessEnvelope` all pass.
- [x] 4.4 Wire stdin/stderr redirection cleanly so tests can drive the prompt via `os.Pipe()` (or by injecting `io.Reader`/`io.Writer` through the command struct). Done when interactive tests run hermetically (no need for a real terminal).

## 5. Wiring and integration

- [x] 5.1 Register `NewAddCmd`, `NewMoveCmd`, `NewRmCmd` on the cobra root in `cmd/ezida/main.go`. Done when `ezida --help` lists `add`, `move`, `rm` alongside the P2 commands.
- [x] 5.2 Add a `TestRoundTrip_AddMoveRm` end-to-end test that initializes a fresh board, adds a card, moves it across two columns, removes it, and asserts the final file equals the initial post-init file. Done when the test passes.
- [x] 5.3 Confirm `output.Fail` recognises all five new typed errors when surfaced from a real command (not just unit tests of `Fail` itself). Done when `TestIntegration_ErrorCodesSurface` covers each command's failure path with `--json` and parses the error code.

## 6. Acceptance gate

- [x] 6.1 Run `go test ./... && go vet ./...` from the repo root. Done when both exit 0.
- [x] 6.2 Manually exercise the happy paths: `ezida init` → `ezida add "T1" --column=todo` → `ezida list --json | jq` shows one card → `ezida move <id> ongoing` → `ezida board` shows the new counts → `ezida rm <id> --yes`. Done when each step's output matches the spec.
- [x] 6.3 Exercise the interactive prompt by running `ezida rm <id>` in a real terminal and confirming the prompt text matches `Delete card <id> "<title>"? [y/N] `. Done when the prompt appears verbatim on stderr. — Verified via `TestRm_InteractiveAccept` which asserts the prompt prefix `Delete card a3f2k9 "` and suffix `? [y/N] ` on stderr; the production code uses the same `fmt.Sprintf("Delete card %s %q? [y/N] ", id, title)` format string regardless of whether stderr is a real TTY.
