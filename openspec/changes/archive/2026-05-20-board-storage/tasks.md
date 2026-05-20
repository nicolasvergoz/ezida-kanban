## 1. Module bootstrap

- [x] 1.1 Run `go mod init github.com/nicolasvergoz/ezida-kanban` at the repo root. Done when `go.mod` exists with module path matching ADR §D17.
- [x] 1.2 Add the TOML dependency with `go get github.com/pelletier/go-toml/v2@latest`. Done when `go.mod` lists the dependency and `go.sum` is populated.
- [x] 1.3 Create the `internal/board/` directory with empty `board.go`, `id.go`, `validation.go`, `board_test.go` files (each with the `package board` declaration). Done when `go build ./internal/board` exits 0.

## 2. ID generation

- [x] 2.1 Implement `NewID() string` in `id.go` using `crypto/rand` and the `[0-9a-z]` alphabet per design. Done when calling `NewID()` returns a string matching `^[0-9a-z]{6}$`.
- [x] 2.2 Implement `ErrIDExhausted` sentinel and `NewUniqueID(existing []string) (string, error)` with a 10-attempt retry loop. Done when a unit test exercising both the happy path and the exhausted path passes.
- [x] 2.3 Add `TestNewIDFormat` and `TestNewUniqueIDCollisions` in `board_test.go`. Done when `go test ./internal/board -run "TestNewID|TestNewUniqueID"` exits 0.

## 3. Struct model

- [x] 3.1 Declare `Board`, `BoardConfig`, `Card` structs in `board.go` with the TOML tags from the design. Done when `go vet ./internal/board` exits 0.
- [x] 3.2 Add the supported schema version constant (`const SupportedSchemaVersion = 1`). Done when the constant is referenced by `Load` and `Validate`.

## 4. Validation

- [x] 4.1 Implement `Violation`, `ValidationError` (with `Error() string` method that yields one line per violation), and `SchemaVersionError` in `validation.go`. Done when both types satisfy the `error` interface and `errors.As` finds them.
- [x] 4.2 Implement `Validate(b *Board) *ValidationError` covering all nine rules from the spec (single-pass, collect all violations). Done when each invalid fixture in `testdata/invalid_*.toml` produces the expected violation list.
- [x] 4.3 Create the validation test fixtures under `internal/board/testdata/`: `valid.toml`, `valid_minimal.toml`, plus one `invalid_<rule>.toml` per rule (9 files). Done when `ls internal/board/testdata/` lists 11 files.
- [x] 4.4 Add `TestValidate_Valid`, `TestValidate_RuleX` (one per rule), and `TestValidate_MultipleViolations`. Done when `go test ./internal/board -run "TestValidate"` exits 0.

## 5. Load / Save

- [x] 5.1 Implement `Load(path string) (*Board, error)` in `board.go`: read the file, unmarshal with `toml.Unmarshal`, return `SchemaVersionError` on version mismatch, run `Validate` and return its error if non-nil. Done when loading `testdata/valid.toml` returns a populated board and a nil error.
- [x] 5.2 Implement `Save(path string, b *Board) error` using the temp-file + `os.Rename` pattern from the design. Done when saving a loaded board produces a file whose bytes round-trip through `Load` to an equal `*Board`.
- [x] 5.3 Implement `AppendCardToColumn(b *Board, c Card)` that inserts the card immediately after the last existing card in the same column (or at the end of `b.Cards` if none). Done when both spec scenarios for this requirement pass.
- [x] 5.4 Add `TestLoadSave_RoundTrip` using `testdata/valid.toml`, `TestLoad_SchemaVersionMismatch`, and `TestSave_AtomicTempFileCleanup`. Done when `go test ./internal/board -run "TestLoad|TestSave"` exits 0.

## 6. Card-order spike

- [x] 6.1 Add `TestRoundTrip_PreservesCardOrder` that constructs a board with 5 cards in column `todo` (IDs `aaaaaa`, `bbbbbb`, `cccccc`, `dddddd`, `eeeeee`), marshals via `Save`, re-loads via `Load`, and asserts `b.Cards` IDs equal the original sequence. Done when the test passes against the chosen TOML library version.
- [x] 6.2 If 6.1 fails, document the failure in a new section of `design.md` titled "Spike outcome — slice order" and add a serializer pass that re-orders `[[cards]]` blocks before write. Done when 6.1 passes with the workaround in place. (Skip if 6.1 passes natively — note the result in the same section.)

## 7. Acceptance gate

- [x] 7.1 Run `go test ./...` from the repo root and confirm all tests pass. Done when exit code is 0 and the output lists every test name from sections 2, 4, 5, and 6.
- [x] 7.2 Run `go vet ./...` and confirm zero diagnostics. Done when exit code is 0.
- [x] 7.3 Commit the change set so the next phase can build on a stable base. (No git step here — handled outside the OpenSpec apply flow.)
