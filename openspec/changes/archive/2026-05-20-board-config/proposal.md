## Why

P3 finished the per-card daily workflow. P4 closes the gap on the rest
of brief §6: `ezida edit` for partial-update on a card, plus
`ezida columns add|rename|rm` and `ezida priorities add|rename|rm` for
board configuration. After P4, the CLI surface is feature-complete for
v1 and only the skill packaging (P5) and distribution (P6) remain.

The non-trivial bits are the two propagation rules from brief §7.4 and
§7.5: a `rename` must atomically rewrite every referencing card, and a
`rm` must refuse when at least one card still references the target.
The error payload for the refusal lists every offending card with ID
and title (ADR §D14) so the user can act without a follow-up `list`.

## What Changes

- Add `ezida edit <id>`:
  - Optional flags `--title`, `--description`, `--priority`, `--tags`,
    `--column`.
  - At least one flag is required. Each provided flag updates the
    corresponding field; unprovided flags leave the card untouched.
  - `--priority` accepts an empty string (`--priority=""`) to clear the
    priority (set to "" in memory → omitted in the marshaled TOML
    thanks to the existing `omitempty` tag).
  - `--tags` always replaces the full tag list (no add/remove flag).
  - `--column` triggers the same "delete then append at bottom" logic
    as `move` to keep the card ordering rule uniform.
  - Always refreshes `updated_at`. Re-validates and saves atomically.
- Add `ezida columns add <name>`:
  - Optional `--position=N` (1-indexed). Defaults to append at end.
  - Refuses if `<name>` is already in the column list (`DUPLICATE`).
  - Refuses with `POSITION_OUT_OF_RANGE` if `--position` is below 1 or
    above `len(columns)+1`.
- Add `ezida columns rename <old> <new>`:
  - Updates `[board].columns` and every card whose `column` equals
    `<old>` in a single write.
  - Refuses if `<old>` is unknown (`COLUMN_NOT_FOUND`) or `<new>` is
    already present (`DUPLICATE`).
- Add `ezida columns rm <name>`:
  - Refuses with `COLUMN_IN_USE` if at least one card has
    `column == <name>`. The error's `details.cards` is a list of
    `{"id","title"}` pairs.
  - Refuses with `LAST_COLUMN` if removing would leave
    `[board].columns` empty (brief field rule: non-empty).
- Add `ezida priorities add <name>`, `rename`, `rm` mirroring the
  columns commands (sharing 95% of the implementation through
  parameterized helpers).
- Add error codes `COLUMN_IN_USE`, `PRIORITY_IN_USE`, `DUPLICATE`,
  `POSITION_OUT_OF_RANGE`, `LAST_COLUMN`, `LAST_PRIORITY`,
  `NOTHING_TO_EDIT` to the enumeration, and wire them into
  `output.Fail`.

## Capabilities

### New Capabilities
- `board-config`: the `edit`, `columns add|rename|rm`, and
  `priorities add|rename|rm` commands, the propagation rules, the
  refusal-with-detail payload, and the new error codes.

### Modified Capabilities
- `card-reading`: extend the error code enumeration with the new P4
  codes. (Same pattern as P3's extension.)

## Impact

- New code: `internal/commands/edit.go`, `internal/commands/columns.go`,
  `internal/commands/priorities.go`, `internal/commands/refgroup.go`
  (shared helpers for the columns / priorities operations), additions
  to `internal/commands/errors.go`, additions to
  `internal/output/exit.go`.
- No new external dependency.
- Every modifying operation re-validates before save, so the file is
  never observed in an inconsistent state.
- `rename` is a single `board.Save` call → atomic by inheritance of
  the temp-file + rename pattern. No additional locking.
- After this phase the CLI matches brief §6 in full.
