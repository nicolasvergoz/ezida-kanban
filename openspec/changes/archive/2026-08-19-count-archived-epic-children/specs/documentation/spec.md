## ADDED Requirements

### Requirement: `docs/usage.md` states how archived children count toward epic progress

`docs/usage.md`'s epics section SHALL state that an epic's derived
progress counts its archived children, and SHALL name the rule that
decides whether one counts as done: the column recorded in the archive,
checked against the board's terminal columns at read time.

The known-limitations section SHALL record the consequence of resolving
done-ness at read time — that deleting, renaming, or un-marking the
column an archived child came from drops it out of `done` while it keeps
counting toward `total` — so a user who hits it can recognise it as
designed behaviour rather than a defect.

#### Scenario: usage.md states that archived children count

- **WHEN** a reader reads the epics section of `docs/usage.md`
- **THEN** it states that archived children are counted in `done` and
  `total`
- **AND** it states that an archived child counts as done when the
  column it was archived from is currently a terminal column

#### Scenario: usage.md records the deleted-column caveat

- **WHEN** a reader reads the known-limitations section
- **THEN** it states that removing or un-marking the column an archived
  child came from stops that child counting toward `done`
- **AND** it notes that the child still counts toward `total`
