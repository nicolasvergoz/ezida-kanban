## 1. Error types and dispatcher

- [x] 1.1 Extend `internal/commands/errors.go` with `ColumnInUseError`, `PriorityInUseError`, `DuplicateError`, `PositionOutOfRangeError`, `LastColumnError`, `LastPriorityError`, `NothingToEditError`. Each implements `Error() string` per the design. Done when `go build ./...` passes and unit tests confirm each `Error()` output.
- [x] 1.2 Add the `affectedCard{ID,Title}` struct (JSON-tagged) and use it inside `ColumnInUseError.Cards` and `PriorityInUseError.Cards`. Done when JSON marshal of an instance produces `{"id":"...","title":"..."}` pairs.
- [x] 1.3 Add the `detailedError` interface (`Code() string`, `Details() any`, `ShortMessage() string`) and implement it on all seven new errors. Done when `errors.As(err, new(detailedError))` succeeds for each.
- [x] 1.4 Wrap existing P2/P3 errors with a tiny `defaultError` adapter exposing the same interface so `output.Fail` only needs one branch path. Done when every existing error code still emits correctly after the refactor (`go test ./internal/commands -run TestOutputFail` passes).
- [x] 1.5 Update `output.Fail`: in text mode emit `Err.Error()` verbatim prefixed by `Error: `; in JSON mode emit `{"error":{"code":Err.Code(),"message":Err.ShortMessage(),"details":Err.Details()}}`. Done when `TestOutputFail_DetailedErrors_TextAndJSON` covers each new code.

## 2. `ezida edit`

- [x] 2.1 Create `internal/commands/edit.go` with `NewEditCmd(jsonOut *bool)` exposing `--title`, `--description`, `--priority`, `--tags`, `--column`. Done when `ezida edit --help` lists all five.
- [x] 2.2 Add a helper that builds `editFlags` (pointer fields) from `cmd.Flags().Changed("...")`. Done when a unit test confirms unset flags are nil and set-but-empty flags resolve to non-nil pointers to `""`.
- [x] 2.3 Implement `runEdit` per the design: enforce at least one flag, then apply mutations via `mutateAndSave` including the column-change re-ordering. Done when `TestEdit_HappyPath`, `TestEdit_MultipleFields`, `TestEdit_NoFlags`, `TestEdit_ClearPriority`, `TestEdit_ChangeColumnReOrders`, `TestEdit_InvalidPriority`, `TestEdit_EmptyTitleRejected`, `TestEdit_JSONEchoesCard` all pass.

## 3. Shared `refgroup` helper

- [x] 3.1 Create `internal/commands/refgroup.go` with the `refGroup` struct and methods `add`, `rename`, `remove` per the design. Each method operates on `*board.Board` passed in; it does NOT load or save. Done when unit tests cover all branches against in-memory boards (no file I/O).
- [x] 3.2 Cover edge cases in unit tests: position 1 / position `len+1` / `len+2` / 0 (out of range), rename to self, rename of unused name, remove with zero / one / many violators, remove of last entry. Done when `go test ./internal/commands -run TestRefGroup` exits 0.

## 4. `ezida columns`

- [x] 4.1 Create `internal/commands/columns.go` with `NewColumnsCmd(jsonOut *bool)` parent + three subcommands (`add`, `rename`, `rm`). `add` carries `--position` (default 0 = append at end). Done when `ezida columns --help` lists the three subcommands.
- [x] 4.2 Build the columns-specific `refGroup` constructor: target = `&b.Board.Columns`, accessor = `&c.Column`, `isReferencing = func(v, n) bool { return v == n }`, error labels = `COLUMN_*`. Wire each subcommand's `RunE` to `mutateAndSave` + the appropriate `refGroup` method. Done when `ezida columns add` / `rename` / `rm` round-trip on a fixture.
- [x] 4.3 Tests: `TestColumnsAdd_Append`, `TestColumnsAdd_AtPosition`, `TestColumnsAdd_Duplicate`, `TestColumnsAdd_PositionOutOfRange`, `TestColumnsRename_Propagates`, `TestColumnsRename_UnknownOld`, `TestColumnsRename_DuplicateNew`, `TestColumnsRm_Unused`, `TestColumnsRm_InUse_TextOutput`, `TestColumnsRm_InUse_JSONOutput`, `TestColumnsRm_LastColumn`. Done when `go test ./internal/commands -run TestColumns` exits 0.

## 5. `ezida priorities`

- [x] 5.1 Create `internal/commands/priorities.go` with `NewPrioritiesCmd(jsonOut *bool)` parent + three subcommands. NO `--position` flag on `add`. Done when `ezida priorities add --position=1` exits 1 with cobra's "unknown flag" error.
- [x] 5.2 Build the priorities-specific `refGroup` constructor: target = `&b.Board.Priorities`, accessor = `&c.Priority`, `isReferencing = func(v, n) bool { return v != "" && v == n }`, error labels = `PRIORITY_*` / `INVALID_PRIORITY`. Done when add/rename/rm round-trip on a fixture.
- [x] 5.3 Tests: `TestPrioritiesAdd_*`, `TestPrioritiesRename_Propagates`, `TestPrioritiesRm_InUse`, `TestPrioritiesRm_LastPriority`, `TestPrioritiesRm_IgnoresCardsWithoutPriority` (confirms a card without `priority` does NOT prevent removal of a priority). Done when `go test ./internal/commands -run TestPriorities` exits 0.

## 6. Wiring

- [x] 6.1 Register `NewEditCmd`, `NewColumnsCmd`, `NewPrioritiesCmd` on the cobra root in `cmd/ezida/main.go`. Done when `ezida --help` lists `edit`, `columns`, `priorities` alongside the P2/P3 commands.
- [x] 6.2 Add a `TestRefusalPayload_TextRendering` integration test that runs `ezida columns rm <name>` against a fixture with two referencing cards, captures stderr, and asserts the exact text (Error line + two indented card lines + closing sentence). Done when the test passes.
- [x] 6.3 Add a `TestRefusalPayload_JSONRendering` integration test that runs the same command with `--json`, captures stderr, parses the JSON, and asserts `error.details.cards` is an array of two `{id,title}` objects. Done when the test passes.

## 7. Acceptance gate

- [x] 7.1 Run `go test ./... && go vet ./...` from the repo root. Done when both exit 0.
- [x] 7.2 Manually verify the propagation path: init a fresh board, add three cards in `todo`, run `ezida columns rename todo backlog`, run `ezida list --json | jq -r '.cards[].column'`, confirm every line reads `backlog`. Done when the observation holds.
- [x] 7.3 Manually verify the refusal path: try to remove a column in use, observe the formatted error on stderr matching the spec. Done when the observation holds.
- [x] 7.4 Cross-check the cumulative error code enumeration in `output.Fail` against the spec's P2+P3+P4 list (19 codes total). Done when the list is exhaustive and no code appears twice.
