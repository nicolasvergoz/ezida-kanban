## Why

With P2 the binary can read a board. P3 makes it useful day-to-day by
adding the three commands a developer reaches for hundreds of times:
`add`, `move`, `rm`. Together they cover the full create / move /
delete lifecycle of a card without yet touching board configuration
(columns, priorities — those land in P4).

## What Changes

- Add three commands in `internal/commands/`:
  - `add.go`: `ezida add "<title>" --column=<name> [--priority=<p>] [--tags=t1,t2] [--description="<text>"]`. Generates a unique ID via `board.NewUniqueID`, sets `created_at = updated_at = now (UTC, second precision)`, appends the card to the bottom of the target column via `board.AppendCardToColumn`, validates, saves atomically.
  - `move.go`: `ezida move <id> <column>`. Updates the card's `column` and refreshes `updated_at`. Re-orders the card to the bottom of the new column section in `b.Cards`. Single write.
  - `rm.go`: `ezida rm <id> [--yes]`. In an interactive TTY without `--yes`, prompts `Delete card a3f2k9 "<title>"? [y/N]` and aborts on anything other than `y`/`Y`. With `--yes` or in a non-TTY context (script), proceeds without prompting. JSON mode requires `--yes` (no prompts in non-interactive mode by definition).
- Extend the JSON contract: mutating commands echo the affected card as `{"card":{...}}` when `--json` is set; `rm --json` returns `{"id":"a3f2k9","deleted":true}`.
- Extend the error code enumeration with `COLUMN_NOT_FOUND`, `INVALID_PRIORITY`, `MISSING_TITLE`, `INVALID_TAG`, `INTERACTIVE_REQUIRED` (raised when `rm` is invoked in JSON mode without `--yes`).
- Update `output.Fail` to recognise the new typed errors and assign the right exit codes (all user errors → exit 1).
- Test coverage: each command's text and JSON paths, validation refusals, the interactive prompt's accept/reject paths (using `os.Pipe` to feed stdin), and a full round-trip (`add` → `move` → `rm` → board state matches expectations).

## Capabilities

### New Capabilities
- `card-writing`: the `add`, `move`, `rm` commands, their flags, their
  validation behavior, the interactive prompt for `rm`, the JSON
  envelopes for mutating commands, and the new error codes.

### Modified Capabilities
- `card-reading`: extend the error code enumeration and the
  `output.Fail` dispatcher so the new typed errors map to stable codes.
  No behavior change to existing commands.

## Impact

- New code: `internal/commands/{add,move,rm}.go`, additions to
  `internal/commands/errors.go`, additions to `internal/output/exit.go`.
- No new external dependencies — `os.Stdin.Stat()` for TTY detection
  keeps us stdlib-only per ADR §D1.
- Every mutating command re-validates the whole board before writing,
  so referential integrity (ADR §D3, brief §7) holds invariantly.
- Atomic writes inherited from `board.Save` — no special handling needed.
