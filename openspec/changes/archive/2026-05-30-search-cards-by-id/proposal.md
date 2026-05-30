## Why

Card IDs are frequently referenced inside other cards' descriptions (e.g.
"blocks `f20wbo`"), but the viewer filter only matches title, description,
and tags. There is no way to locate a card by its ID — users have to scan
columns manually. Extending the filter to match card IDs closes that gap
with a tiny, mechanical change.

## What Changes

- Extend `matchCard` in the viewer to test the query against `card.id` in
  addition to title, description, and tags.
- Add an `ID` scope pill to the filter popover, mirroring the existing
  Title / Description / Tags pills (default on).
- Persist the new `inId` flag in the filter state object alongside
  `inTitle`, `inDescription`, `inTags`.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `viewer-ui`: filter scope adds card ID as a fourth matchable field with
  its own toggle pill.

## Impact

- `internal/server/web/app.jsx`: `DEFAULT_FILTER`, `filterIsActive`,
  `matchCard`, and the Filter popover JSX.
- `openspec/specs/viewer-ui/spec.md`: requirement "Filter matches title,
  description, and tags case-insensitively" gains an ID scope.
- No server-side changes. No schema changes. No new dependencies.
