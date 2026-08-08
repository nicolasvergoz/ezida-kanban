## 1. Storage model

- [x] 1.1 Add `Epic string` (`toml:"epic,omitempty"`) and `Color string` (`toml:"color,omitempty"`) to `board.Card` in `internal/board/board.go`
- [x] 1.2 Add a terminal-column set to `BoardConfig` alongside `Columns []string`, exposed through a helper such as `IsDoneColumn(name string) bool` and `DoneColumns() []string` so callers never touch the raw map
- [x] 1.3 Write `DecodeColumns([]string) (names []string, done map[string]bool)` and `EncodeColumns(names []string, done map[string]bool) []string` as pure functions in `internal/board/columns.go`
- [x] 1.4 Call `DecodeColumns` in `Load` after unmarshal and `EncodeColumns` in `Save` before marshal, so `BoardConfig.Columns` always holds bare names in memory
- [x] 1.5 Bump `SupportedSchemaVersion` to `2`
- [x] 1.6 Make `Save` emit the explanatory comment above the `columns` key describing the `*` suffix
- [x] 1.7 Unit-test `DecodeColumns`/`EncodeColumns` as a round-trip pair: zero, one, and multiple markers; a name containing `*` in the middle; the `['done','done*']` collision

## 2. Validation

- [x] 2.1 Add rule 11 (`epic` names an existing card) to `Validate` in `internal/board/validation.go`
- [x] 2.2 Add rule 12 (`epic` is not the card's own id)
- [x] 2.3 Add rule 13 (a card carrying `epic` is not referenced as an `epic`), implemented as a two-pass set check with no traversal
- [x] 2.4 Add rule 14 (`color` matches the hex pattern), reusing `hexColorPattern`
- [x] 2.5 Add rule 15 (`epic` or `color` present requires `schema_version >= 2`)
- [x] 2.6 Add rules 16 and 17 (decoded column names non-empty and `*`-free; decoded names unique)
- [x] 2.7 Make the rule-13 violation message state that the target already belongs to an epic and that nesting is one level, not just that the id was rejected
- [x] 2.8 Extend `internal/board/board_test.go` with one failing fixture per new rule, plus a valid board exercising all of them at once

## 3. Color palette

- [x] 3.1 Create `internal/board/colors.go` with the ordered `EpicPalette` (violet, emerald, orange, blue, pink, lime, cyan, fuchsia) and their hex values
- [x] 3.2 Add `ResolveColor(nameOrHex string) (string, error)` mapping a palette name to its hex and validating a literal hex
- [x] 3.3 Add `AssignColor(b *Board) string` returning the least-used palette color, breaking ties by palette order
- [x] 3.4 Test `AssignColor` for: empty board, partial use, exhausted palette, and the delete-then-create case that a modulo strategy would get wrong
- [x] 3.5 Assert in a test that no palette hex collides with a value in `DefaultPriorityColors`

## 4. Epic helpers

- [x] 4.1 Add `ChildrenOf(b *Board, id string) []Card` returning children in board file order
- [x] 4.2 Add `EpicProgress(b *Board, id string) (done, total int)` counting children whose column is terminal
- [x] 4.3 Add `IsEpic(b *Board, id string) bool` (referenced by at least one card) and `ParentOf(b *Board, id string) *Card`
- [x] 4.4 Extend `CardPatch` with `Epic *string` and `Color *string`, honoring the absent-vs-empty pointer convention
- [x] 4.5 Add the pre-mutation checks to `UpdateCard`: unknown epic, self-reference, and target-is-itself-a-child each return a typed error before any mutation
- [x] 4.6 Test `ChildrenOf` order stability and `EpicProgress` with zero terminal columns

## 5. Card deletion and orphaning

- [x] 5.1 Extend `DeleteCard` (or add a wrapper used by the CLI) to clear `Epic` on every card referencing the deleted id, without refreshing their `UpdatedAt`
- [x] 5.2 Return the orphaned ids in board file order so the command layer can report them
- [x] 5.3 Test that deleting a child leaves the parent byte-unchanged including `Color` and `UpdatedAt`
- [x] 5.4 Test that deleting a parent clears exactly the referencing cards and touches nothing else

## 6. Column commands

- [x] 6.1 Add `ezida columns done <name>` and `ezida columns undone <name>` in `internal/commands/columns.go`, both idempotent and both rejecting a `*` in the argument
- [x] 6.2 Make bare `ezida columns` list columns with counts and a terminal indicator
- [x] 6.3 Make `RenameColumn` carry the terminal marker across the rename, and reject a `to` value containing `*` with `INVALID_COLUMN_NAME`
- [x] 6.4 Test rename preserving the marker, `done`/`undone` idempotence, and the suffix-as-argument rejection

## 7. Card commands

- [x] 7.1 Add `--epic` and `--color` to `ezida add` in `internal/commands/add.go`, assigning the parent a color when it has none
- [x] 7.2 Add `--epic`, `--no-epic`, `--color`, `--no-color` to `ezida edit`, with mutual-exclusion checks
- [x] 7.3 Include the new flags in the `NOTHING_TO_EDIT` "at least one flag" check
- [x] 7.4 Wire the orphaning report into `ezida rm`: `orphaned` array in the JSON envelope, count and ids on stderr in text mode
- [x] 7.5 Add `--epic=<id>` to `ezida list`, matching the parent as well as its children, rejecting an unknown id with `INVALID_FILTER`
- [x] 7.6 Extend `ezida get` to report the parent on a child, and children plus progress on a parent, in both output modes
- [x] 7.7 Add `ezida board` reporting of `done_columns` in JSON and a terminal marker in text, with bare names in both

## 8. Colors command

- [x] 8.1 Add `ezida colors` in `internal/commands/colors.go` listing each palette entry with its holder or `null`
- [x] 8.2 Include off-palette colors currently in use as entries with a `null` name
- [x] 8.3 Test the JSON shape and the held/free distinction

## 9. Migration

- [x] 9.1 Add `ezida migrate` in `internal/commands/migrate.go`, decoding the TOML directly rather than through `Load` so it can read a v1 file
- [x] 9.2 Write `kanban.toml.v1.bak` before any mutation, aborting with `IO_ERROR` if the backup fails
- [x] 9.3 Choose the terminal column: a column named `done` if present, otherwise the last declared one
- [x] 9.4 Run `Validate` on the upgraded board before `Save`, aborting with `VALIDATION_FAILED` on any violation
- [x] 9.5 Report the source version, target version, and chosen terminal column in both output modes
- [x] 9.6 Reject an already-v2 file with `MIGRATION_NOT_NEEDED` and a future version with `SCHEMA_VERSION_MISMATCH`
- [x] 9.7 Remind the user to run `ezida init --skill-only` in the success output
- [x] 9.8 Test the full upgrade against a real v1 fixture, asserting every card field is byte-unchanged

## 10. Error surface

- [x] 10.1 Add `INVALID_EPIC`, `INVALID_COLOR`, `INVALID_COLUMN_NAME`, `MIGRATION_NOT_NEEDED` to the error-code enumeration in `internal/output/exit.go`
- [x] 10.2 Make `INVALID_EPIC` carry the rejected id in `error.details`
- [x] 10.3 Update the `SCHEMA_VERSION_MISMATCH` message to name `ezida migrate` when the file is older and to direct to a binary upgrade when it is newer
- [x] 10.4 Extend `internal/commands/errors_test.go` with a case per new code

## 11. Init and output shapes

- [x] 11.1 Make `ezida init` write `schema_version = 2`, mark one column terminal by the same rule as `migrate`, and emit the explanatory comment
- [x] 11.2 Make `ezida init --columns` honor explicit `*` suffixes and suppress the automatic choice when any are present
- [x] 11.3 Add `epic` and `color` (both `omitempty`) to `output.ListCard` and `output.GetCard`; add `epic`, `children`, `progress` to the `get` envelope
- [x] 11.4 Add `done_columns` to `output.BoardEnvelope`
- [x] 11.5 Leave `output.ExportCard` and `internal/server/` untouched — assert this with a test or a review checklist item, since the drift is deliberate and time-boxed to the next change

## 12. Skill and docs

- [x] 12.1 Update `internal/skill/SKILL.md` with the epic and color flags, `ezida columns done|undone`, `ezida colors`, and `ezida migrate`
- [x] 12.2 Teach the skill the one-level rule explicitly, so Claude does not attempt nested epics
- [x] 12.3 Update `docs/usage.md`: new flags on `add`/`edit`/`list`/`get`, the new commands, the `*` suffix, and a migration section
- [x] 12.4 Update `README.md` if it states the schema version or the card field list

## 13. Verification

- [x] 13.1 Run `gofmt -l .` and `go vet ./...` clean
- [x] 13.2 Run `go test ./...` green
- [x] 13.3 Manually migrate this repo's own `kanban.toml`, then create an epic across three of its real cards and confirm `ezida get` reports the right progress
- [x] 13.4 Confirm `ezida serve` still boots and renders the board unchanged, proving the frontend was untouched
