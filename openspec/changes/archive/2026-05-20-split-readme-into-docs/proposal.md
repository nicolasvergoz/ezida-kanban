## Why

`README.md` has grown to 355 lines (~11 KB) and mixes a beginner's
"what is this / how do I install" with deep CLI reference, JSON schema
documentation, embedded-skill internals, and release procedure. New
readers must scroll past pages of reference material before finding
the quick start, and contributors must scan a single dense file to
find the section they need.

## What Changes

- Slim `README.md` down to the essentials a first-time reader needs:
  intro/tagline, one-liner install, quick start, license, and clearly
  signposted links to `docs/`.
- Add `docs/usage.md`: full CLI reference (every subcommand and flag),
  JSON contract, embedded-skill details, known limitations, and the
  manual install procedure.
- Add `docs/development.md`: contributing notes (test commands,
  OpenSpec workflow) and the release procedure.
- No code, behaviour, or spec changes. No CLI flags or JSON envelopes
  are added, removed, or renamed.

## Capabilities

### New Capabilities

<!-- None. This change is documentation-only and touches no behavioural capability. -->

### Modified Capabilities

<!-- None. README/docs are not governed by a spec. No requirements change. -->

## Impact

- Files affected: `README.md` (rewritten/shortened), `docs/usage.md`
  (new), `docs/development.md` (new).
- No code, no tests, no CI, no public commands change.
- External docs/links may target old README anchors; the new README
  retains anchors only for sections still present (install, quick
  start, license). All other deep-link anchors move under `docs/`.
