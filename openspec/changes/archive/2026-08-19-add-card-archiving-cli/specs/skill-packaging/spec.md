## ADDED Requirements

### Requirement: The embedded skill lists the archive verbs

The canonical skill file SHALL teach the archive surface, so that an agent
reading it knows archiving exists and never resorts to deleting a finished
card. The listing MUST cover `ezida archive <id>`, `ezida archive column
<name>`, `ezida archive list`, `ezida archive get <id>` and `ezida unarchive
<id>`, and MUST state that archiving an epic takes its children with it.

Both copies of the skill — the embedded source of truth and the copy `ezida
init` writes — MUST remain byte-identical.

#### Scenario: The skill names every archive verb

- **WHEN** the embedded skill content is read
- **THEN** it contains `ezida archive`
- **AND** it contains `ezida unarchive`
- **AND** it states that archiving an epic also archives its children

#### Scenario: The two skill copies stay identical

- **WHEN** the embedded skill bytes are compared with the on-disk skill file
  after this change
- **THEN** the two MUST be byte-identical
