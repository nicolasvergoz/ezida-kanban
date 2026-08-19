## MODIFIED Requirements

### Requirement: ID format and generation

Card IDs SHALL be exactly six characters drawn uniformly from the alphabet
`[0-9a-z]`. The package MUST expose `NewID() string` for unconditional
generation and `NewUniqueID(existing []string) (string, error)` that retries
up to ten times against the provided set and returns `ErrIDExhausted` on
exhaustion.

Uniqueness SHALL span the board **and** its archive. Every caller that mints an
ID MUST pass the union of live card IDs and archived card IDs as `existing`, so
that a newly created card can never collide with a card waiting to be restored.
The package MUST expose a helper that computes this union from a board and an
archive, tolerating a nil archive.

#### Scenario: NewID format

- **WHEN** `NewID()` is called
- **THEN** the returned string MUST match the regular expression
  `^[0-9a-z]{6}$`

#### Scenario: NewUniqueID avoids collisions

- **WHEN** `NewUniqueID(existing)` is called with a non-empty `existing`
  slice
- **THEN** the returned ID MUST NOT appear in `existing`

#### Scenario: NewUniqueID gives up after ten attempts

- **WHEN** `NewUniqueID` is invoked against a synthetic `existing` set
  that covers all 36⁶ values
- **THEN** the function MUST return `ErrIDExhausted`

#### Scenario: A new card cannot collide with an archived card

- **WHEN** a card is created against a board whose archive already contains
  the ID the generator would otherwise return first
- **THEN** the created card's ID MUST differ from that archived ID

#### Scenario: The union helper tolerates an absent archive

- **WHEN** the union of existing IDs is computed for a board with no archive
- **THEN** the result MUST equal the board's own card IDs
