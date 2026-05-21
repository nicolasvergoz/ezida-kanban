## ADDED Requirements

### Requirement: `UpdateCard` applies a partial patch to a card

The package SHALL expose `UpdateCard(b *Board, id string, p CardPatch) error` that mutates the card identified by `id` according to `p`. Each non-nil field in `p` MUST replace the corresponding card field; nil fields MUST leave the card field unchanged. The helper MUST refresh `UpdatedAt` to the current UTC time at second precision and MUST call `Validate(b)` after the mutation, returning the validation error if any rule fails.

Pre-mutation rule checks (in order):

- If `p.Title != nil` and the trimmed value is empty, return `*MissingTitleError`.
- If `p.Tags != nil` and any element's trimmed value is empty, return `*InvalidTagError`.
- If `p.Priority != nil` and the value is non-empty but not present in `b.Board.Priorities`, return `*InvalidPriorityError`.

If `id` does not match any card, return `*CardNotFoundError` before any mutation.

#### Scenario: Patch only the title

- **WHEN** a card has `Title="Old"`, `Description="x"`, `Tags=["a"]`, `Priority="low"` and `UpdateCard(b, id, CardPatch{Title: ptr("New")})` is called
- **THEN** the card's `Title` MUST equal `"New"`
- **AND** the card's `Description`, `Tags`, and `Priority` MUST be unchanged
- **AND** the card's `UpdatedAt` MUST be refreshed

#### Scenario: Patch clears a field via empty value

- **WHEN** a card has `Priority="high"` and `UpdateCard(b, id, CardPatch{Priority: ptr("")})` is called
- **THEN** the card's `Priority` MUST equal `""`

#### Scenario: Patch clears tags via empty slice

- **WHEN** a card has `Tags=["a","b"]` and `UpdateCard(b, id, CardPatch{Tags: ptrSlice([]string{})})` is called
- **THEN** the card's `Tags` MUST equal `[]`

#### Scenario: Patch with empty title is rejected

- **WHEN** `UpdateCard(b, id, CardPatch{Title: ptr("   ")})` is called
- **THEN** the call MUST return `*MissingTitleError`
- **AND** the card MUST be unchanged

#### Scenario: Patch with unknown priority is rejected

- **WHEN** `UpdateCard(b, id, CardPatch{Priority: ptr("urgent")})` is called and `urgent` is not in `b.Board.Priorities`
- **THEN** the call MUST return `*InvalidPriorityError`
- **AND** the card MUST be unchanged

#### Scenario: Patch with empty-string tag is rejected

- **WHEN** `UpdateCard(b, id, CardPatch{Tags: ptrSlice([]string{"a", ""})})` is called
- **THEN** the call MUST return `*InvalidTagError`
- **AND** the card MUST be unchanged

#### Scenario: Patch with unknown card id

- **WHEN** `UpdateCard(b, "zzzzzz", any-patch)` is called and no card has id `zzzzzz`
- **THEN** the call MUST return `*CardNotFoundError`
- **AND** no card in `b.Cards` MUST be mutated

### Requirement: `CardPatch` distinguishes "absent" from "empty"

The package SHALL declare `CardPatch` with pointer fields for every patchable card attribute (at minimum: `Title *string`, `Description *string`, `Tags *[]string`, `Priority *string`). Pointer nil MUST mean "absent in this patch"; pointer non-nil MUST mean "explicit value, including the empty value". JSON encoding/decoding MUST honor this distinction via `omitempty` plus pointer presence.

#### Scenario: Unmarshalling JSON with absent key leaves pointer nil

- **WHEN** the JSON `{"title":"hi"}` is unmarshalled into a `CardPatch`
- **THEN** the resulting struct MUST have non-nil `Title` pointing to `"hi"`
- **AND** the resulting struct MUST have nil `Description`, `Tags`, `Priority`

#### Scenario: Unmarshalling JSON with empty value yields non-nil empty pointer

- **WHEN** the JSON `{"tags":[]}` is unmarshalled into a `CardPatch`
- **THEN** the resulting struct MUST have non-nil `Tags` pointing to an empty slice
