## MODIFIED Requirements

### Requirement: `ezida columns rm` refuses when in use

`ezida columns rm <name>` SHALL remove the column from
`[board].columns` ONLY when no card references it. Otherwise, the
command MUST refuse with `COLUMN_IN_USE` and a payload listing every
offending card.

The text-mode error MUST be:
```
Error: column "todo" still referenced by N cards:
  <id1>  <title1>
  <id2>  <title2>
Move or remove these cards first, or archive them with `ezida archive column todo`.
```
The JSON-mode error MUST include
`"details":{"column":"<name>","cards":[{"id":"...","title":"..."}]}`.

The remedy naming `ezida archive column <name>` MUST also appear in the
message carried by the board-layer refusal, so that consumers other than the
CLI surface the same guidance.

If removing the column would leave `[board].columns` empty, the
command MUST refuse with `LAST_COLUMN`.

#### Scenario: Remove unused column

- **WHEN** `ezida columns rm review` is invoked when no card
  references `review`
- **THEN** the process exits with code `0`
- **AND** `[board].columns` no longer contains `review`

#### Scenario: Refuse when cards reference the column

- **WHEN** `ezida columns rm todo` is invoked while 2 cards have
  `column = "todo"`
- **THEN** the process exits with code `1`
- **AND** the error code (JSON mode) is `COLUMN_IN_USE`
- **AND** the JSON error's `details.cards` lists both `{id, title}`
  pairs
- **AND** the text-mode message lists both cards as
  `  <id>  <title>` (two-space indent per line)
- **AND** the text-mode message names `ezida archive column todo` as a remedy
- **AND** `kanban.toml` is byte-unchanged

#### Scenario: Refuse to remove the last column

- **WHEN** `ezida columns rm todo` is invoked against a board whose
  `[board].columns` is `["todo"]` and where no card references `todo`
- **THEN** the process exits with code `1`
- **AND** the error code is `LAST_COLUMN`

#### Scenario: Refuse to remove an unknown column

- **WHEN** `ezida columns rm ghost` is invoked
- **THEN** the process exits with code `1`
- **AND** the error code is `COLUMN_NOT_FOUND`

#### Scenario: Archiving the column clears the refusal

- **WHEN** `ezida archive column todo` is invoked and then
  `ezida columns rm todo`
- **THEN** the second command exits with code `0`
- **AND** `[board].columns` no longer contains `todo`
