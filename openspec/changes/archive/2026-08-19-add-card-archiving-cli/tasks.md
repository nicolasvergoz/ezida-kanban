Every group ends green under `./scripts/verify.sh --go`. No browser is needed
anywhere in this change.

## 1. Archive types and IO

- [x] 1.1 Create `internal/board/archive.go` with `Archive{SchemaVersion, Cards}` and `ArchivedCard{Card; ArchivedAt time.Time \`toml:"archived_at"\`}` — `Card` embedded anonymously, no tag
- [x] 1.2 Add `ArchivePathFor(boardPath string) string` deriving the sibling `kanban.archive.toml`
- [x] 1.3 Add `LoadArchive(path string) (*Archive, bool, error)` — a missing file yields an empty archive at `SupportedSchemaVersion` and `existed=false`, not an error
- [x] 1.4 Add `SaveArchive(path string, a *Archive) error` — validate first, then atomic temp-in-same-dir + rename with prefix `.kanban.archive.toml.tmp.*`; **remove the file instead of writing when the archive has no cards**
- [x] 1.5 Add `ExistingIDs(b *Board, a *Archive) []string` (nil-archive tolerant) and `ReconcileArchive(b *Board, a *Archive) []string` (drops archived cards whose id is live, returns dropped ids in archive order, pure)
- [x] 1.6 Add `CardNotArchivedError{ID}` and `IDCollisionError{ID}` alongside the existing board error types
- [x] 1.7 Write `internal/board/archive_test.go`: `TestArchivedCard_EmbedsCardVerbatim` (reflect — field 0 anonymous of type `Card`; recursive toml-tag set equals tags(`Card`) ∪ `archived_at`), `TestArchive_TOMLRoundTrip` (every field preserved; `archived_at` is a flat key inside `[[cards]]`; unset optionals absent), `TestLoadArchive_MissingFileIsEmptyNotError`, `TestSaveArchive_RemovesFileWhenEmpty`, `TestArchivePathFor` (table), `TestReconcileArchive_BoardWins`

## 2. Archive validation

- [x] 2.1 Create `internal/board/archive_validation.go` with `ValidateArchive(a *Archive) *ValidationError`, collecting all violations in one pass and reusing `Violation`
- [x] 2.2 Implement kept rules 1, 4, 5 (unique within the archive), 6, 9, 12, 14
- [x] 2.3 Implement added rule 18 (`archived_at` non-zero and not before `created_at`) and rule 19 (`column` non-empty)
- [x] 2.4 Add archive fixtures under `internal/board/testdata/`
- [x] 2.5 Write `internal/board/archive_validation_test.go`, one test per kept and added rule, plus the two that pin the deliberate divergence: `TestValidateArchive_AllowsUnknownColumn` and `TestValidateArchive_AllowsDanglingEpic`

## 3. Archive operations

- [x] 3.1 Create `internal/board/archive_ops.go` with `ArchiveCard(b, a, id, at) ([]ArchivedCard, error)` — set is `{id} ∪ ChildrenOf(b, id)`, inserted at the head parent-first then children in board file order, one shared `archived_at`, `updated_at` untouched, children's `epic` preserved; pure, never persists
- [x] 3.2 Add `ArchiveColumn(b, a, column, at) (direct, cascaded []ArchivedCard, err error)` — `direct` are the column's cards, `cascaded` are their epic children living elsewhere; the two are disjoint; unknown column returns `*ColumnNotFoundError`
- [x] 3.3 Add `UnarchiveCard(b, a, id, column) (restored []Card, orphaned []string, relocated bool, err error)` — restores `{id} ∪ archived children of id` in reverse archive order via `PrependCardToColumn`; explicit `column` wins, else the stored column, else the board's first column with `relocated=true`; clears `epic` on cards whose parent is neither live nor restored and reports them; refuses `*IDCollisionError` when the id is already live
- [x] 3.4 Write `internal/board/archive_ops_test.go`: `TestArchiveCard_CascadesChildren` (set and order), `_SharesOneTimestamp`, `_DoesNotTouchUpdatedAt`, `_LoneChildKeepsEpicString`, `TestArchiveColumn_CascadeOutsideColumnIsReportedSeparately`, `TestUnarchiveCard_RestoresCascadeInBoardOrder`, `_ClearsEpicWhenParentGone`, `_FallsBackToFirstColumn`, `_RefusesIDCollision`, and `TestArchiveUnarchive_RoundTripIsIdentity`

## 4. Command helpers and ID widening

- [x] 4.1 Add `const ArchivePath = "kanban.archive.toml"` to `internal/commands/init.go` beside `BoardPath`, with a test asserting `board.ArchivePathFor(BoardPath) == ArchivePath`
- [x] 4.2 Create `internal/commands/archive_helpers.go` with `loadArchive(path) (*board.Archive, error)` and the two save-order helpers `mutateArchiveAndSave` (archive written first) and `mutateUnarchiveAndSave` (board written first), mirroring `mutateAndSave`'s contract
- [x] 4.3 Change `internal/commands/add.go` to mint ids against `board.ExistingIDs(b, a)`
- [x] 4.4 Change `internal/server/handlers.go` `handleCreate` to do the same — the server shares the mint path and would otherwise reintroduce the collision
- [x] 4.5 Tests: `TestAdd_DoesNotCollideWithArchivedID`, `TestCreateCard_DoesNotCollideWithArchivedID`, `TestMutateArchiveAndSave_WritesArchiveBeforeBoard` (inject a failing board save; assert the card is in both files — the documented duplicate, not a loss)

## 5. `ezida archive` command tree

- [x] 5.1 Create `internal/commands/archive.go` with `NewArchiveCmd(jsonOut *bool)` — parent takes `cobra.MaximumNArgs(1)` and dispatches `archive <id>`, with unexported `newArchiveColumnCmd` / `newArchiveListCmd` / `newArchiveGetCmd` children, following the `columns` precedent (list/get registration deferred to §7, alongside the list/get machinery they wrap)
- [x] 5.2 Implement `runArchive` — cascade, both writes, `CARD_NOT_FOUND` on an unknown id with both files unchanged; stdout is the id alone; stderr reports cascade and the lone-child note
- [x] 5.3 Implement `runArchiveColumn` with `--yes` — prompt via `promptConfirm` and the `rmIO` injection when the cascade leaves the column; `INTERACTIVE_REQUIRED` for `--json` without `--yes`; no prompt when nothing outside the column is affected; empty column is a no-op that writes nothing
- [x] 5.4 Add the JSON envelopes as structs (key order is contractual): `{id, archived, cascaded}` with `cascaded` never null
- [x] 5.5 Register `NewArchiveCmd` in `cmd/ezida/main.go`
- [x] 5.6 Write `internal/commands/archive_test.go`: `TestArchive_MovesCardOutOfBoard`, `_CascadeReportsChildrenOnStderr`, `_JSONEnvelopeShape`, `_UnknownID_CardNotFound`, `TestArchiveColumn_PromptsWhenCascadeLeavesColumn`, `_JSONWithoutYes_InteractiveRequired`, `_EmptyColumnWritesNothing` (board byte-unchanged and no archive file created), `_LeavesColumnInPlace`

## 6. `ezida unarchive`

- [x] 6.1 Create `internal/commands/unarchive.go` with `NewUnarchiveCmd(jsonOut *bool)` and `--column`; envelope `{id, unarchived, cascaded, orphaned, column, relocated}`
- [x] 6.2 Report relocation, cascade and orphaning on stderr in text mode
- [x] 6.3 Register in `cmd/ezida/main.go`
- [x] 6.4 Write `internal/commands/unarchive_test.go`: `TestUnarchive_RestoresCascade`, `_DeletesArchiveFileWhenLastCardLeaves`, `_RelocatesWhenColumnGone`, `_ColumnFlagOverride`, `_UnknownExplicitColumn`, `_UnknownID_CardNotArchived`
- [x] 6.5 Add `TestRoundTrip_ArchiveUnarchive` to `internal/commands/roundtrip_test.go` — init → add → archive → unarchive leaves `kanban.toml` byte-identical to post-add **and** no `kanban.archive.toml` on disk

## 7. Reading the archive from `list` and `get`

- [x] 7.1 Add `ArchivedAt *time.Time \`json:"archived_at,omitempty"\`` to `output.ListCard` and `output.GetCard` — a pointer, so a zero timestamp cannot leak onto live cards
- [x] 7.2 Add `--include-archived` and `--archived-only` to `listFlags`; hand-roll the exclusion check with a new `MutuallyExclusiveFlagsError` (`MUTUALLY_EXCLUSIVE_FLAGS`, exit 1) rather than cobra's `MarkFlagsMutuallyExclusive`
- [x] 7.3 Order merged results live-then-archived, each in file order; append the `ARCHIVED` text column only when one of the two flags is set
- [x] 7.4 Widen `--column` / `--priority` / `--epic` validity to board ∪ archive values under `--archived-only`
- [x] 7.5 Wire `archive list` and `archive get` as thin wrappers over the same `runList` / `runGet` machinery; unknown id in `archive get` is `CARD_NOT_ARCHIVED` (implemented as `runArchiveGet`, since a bare `runGet` reuse would resolve epic/children against the wrong graph)
- [x] 7.6 Tests: `TestList_DefaultOutputUnchangedWithArchivePresent` (byte-identical stdout), `_IncludeArchived_OrderIsLiveThenArchived`, `_ArchivedOnly_AcceptsDeletedColumnFilter`, `_BothFlags_MutuallyExclusive`, `TestList_JSONOmitsArchivedAtForLiveCards` (decode to `map[string]any`, assert the key is absent), `TestArchiveGet_LiveCardIsNotArchived`

## 8. Error taxonomy wiring

- [x] 8.1 Add `CardNotArchivedError` and `MutuallyExclusiveFlagsError` to `internal/commands/errors.go` in the legacy `CodedError` shape used by their neighbours (added `IDCollisionError` too, plus a shared `asBoardError` translator for the board package's own untyped-for-CLI errors)
- [x] 8.2 Add both, plus `ID_COLLISION`, to the `TestErrors_TypedErrorsImplementCodedError` table in `errors_test.go`
- [x] 8.3 Add end-to-end rows to `TestIntegration_ErrorCodesSurface` in `integration_errors_test.go`

## 9. `columns rm` remedy

- [x] 9.1 Reword `board.ColumnHasCardsError.Error()` (`columns.go:148`) to name `ezida archive column <name>`
- [x] 9.2 Reword `commands.ColumnInUseError` identically
- [x] 9.3 Update the asserted literal at `errors_test.go:80` and any server test asserting the message (also `output_test.go`, `columns_test.go`, `refusal_integration_test.go` — no server-layer test asserted the board message literal)
- [x] 9.4 Add `TestColumnInUse_SuggestsArchiveColumn` and `TestColumnsRm_SucceedsAfterArchivingTheColumn`

## 10. Documentation and skill

- [x] 10.1 Add `### ezida archive` and `### ezida unarchive` sections to the CLI reference in `docs/usage.md`, covering all four archive forms, `--yes` and `--column`
- [x] 10.2 Document `--include-archived` / `--archived-only` under `### ezida list`, including that they cannot be combined
- [x] 10.3 Document `archived_at` in the JSON contract section (both `ezida list --json` and a new `### ezida archive get --json` section), stating it is omitted entirely for live cards
- [x] 10.4 Add the two-file atomicity caveat and the `archive column` shadowing caveat to the known-limitations section
- [x] 10.5 Update `internal/skill/SKILL.md` and `.claude/skills/ezida-kanban/SKILL.md` **byte-identically** — reading list, writing list, and a new `## Archiving` section — naming every archive verb and stating that archiving an epic takes its children
- [x] 10.6 Run `go test ./internal/skill/...` to confirm the two copies still match

## 11. Final gate

- [x] 11.1 `gofmt -l .` clean, `go vet ./...` clean
- [x] 11.2 `./scripts/verify.sh --go` green
- [x] 11.3 Manual smoke: `init` → `add` → `archive` → inspect both files → `archive list` → `list --include-archived` → `unarchive` → confirm `kanban.archive.toml` is gone and `kanban.toml` matches its pre-archive bytes
