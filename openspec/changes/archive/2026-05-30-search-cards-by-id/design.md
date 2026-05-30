## Context

The viewer (`internal/server/web/app.jsx`) renders a Filter popover whose
state is a plain object with a free-text `query` and three boolean scope
toggles: `inTitle`, `inDescription`, `inTags`. `matchCard(card, f)` walks
those scopes and returns true on the first hit. Card IDs are short opaque
strings (e.g. `f20wbo`) emitted by the server and surfaced in the card
header via `CopyableId`. Users routinely paste these IDs into other cards'
descriptions to express references and dependencies, but the filter
cannot match them.

## Goals / Non-Goals

**Goals:**
- Filter matches `card.id` substring case-insensitively when an `ID`
  scope is enabled.
- New `ID` pill in the popover, default on, behaves identically to the
  other scope pills.

**Non-Goals:**
- No server-side filtering.
- No exact-ID jump / "go to card" UX. Substring match is enough.
- No keyboard shortcut for ID search.
- No URL persistence (the filter remains transient per existing spec).

## Decisions

**Add a fourth scope flag (`inId`) rather than hard-wiring ID matching.**
Mirrors the existing pattern so users can opt out (e.g. when a 2-letter
query collides with many IDs). Cost is one extra pill and one boolean —
trivial. Alternative considered: match ID unconditionally whenever the
query is non-empty. Rejected because it would expand the result set in a
way that surprises users and cannot be turned off.

**Default `inId: true`.** Symmetry with the other three scopes; the
feature is discoverable on first use without forcing the user to enable
it.

**Reuse the same substring + lowercase logic.** No special handling for
ID prefix matches or exact matches. Keeps `matchCard` a single short
function.

## Risks / Trade-offs

- [Short queries (1–2 chars) may now match many IDs by accident] →
  Mitigation: scope pill allows users to disable ID matching; behavior is
  consistent with how 1–2 char queries already over-match tags.
- [`app.js` legacy bundle drifts further from `app.jsx`] → Not a risk:
  `index.html` and `site/demo/index.html` both load `app.jsx` through
  Babel-standalone; `app.js` is no longer served.
