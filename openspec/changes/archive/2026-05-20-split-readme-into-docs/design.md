## Context

`README.md` is the project's front door but currently does double duty
as reference manual. At 355 lines it contains: a 4-line tagline, two
install paths, quick-start examples, a full CLI subcommand reference
with flag tables, JSON envelope examples for `board`/`list`/`get`,
error-envelope schema, an embedded-skill explanation, a known-limits
list, a contributing section, a release procedure, and the license.

Most of this is reference content that only matters once the reader
has installed the tool. Putting it on the landing page makes the
"what is this / how do I try it" loop slow and noisy.

## Goals / Non-Goals

**Goals:**
- Reduce the README to the minimum a first-time visitor needs to
  decide "is this for me?" and "how do I install it?".
- Move reference and operational content under `docs/` so it's easy
  to find by topic without scrolling past it on every visit.
- Keep all current content — nothing is removed, only relocated.
- Preserve correctness of every command/flag/JSON shape documented;
  this is a move, not a rewrite of facts.

**Non-Goals:**
- No new behaviour, flags, or JSON fields. No spec changes.
- Not splitting `docs/` into one-file-per-topic. We use two flat
  files (`usage.md`, `dev.md`-ish) deliberately to avoid premature
  directory sprawl on a young project.
- Not setting up a docs site (mkdocs / docusaurus / etc.). Markdown
  on GitHub is enough at this stage.
- Not maintaining redirects from old README anchors. Anchors that no
  longer exist in the README will 404 at the fragment — acceptable
  for a v0.x project.

## Decisions

### Decision 1 — README scope: lean

Keep in `README.md`:
- Title, tagline, supported platforms.
- One-liner install (curl + `EZIDA_VERSION` pin), nothing more.
- Quick start (`ezida init` + 3-4 example commands).
- Two prominent links: `docs/usage.md` and `docs/development.md`.
- License section.

Move out: manual install, full CLI reference, JSON contract,
embedded-skill internals, known limitations, contributing, releasing.

**Rationale:** the README answers "what / install / try"; everything
else is reference and belongs in topic-scoped docs.

**Alternative considered:** keep known-limitations in README so users
see caveats before installing. Rejected — the caveats are minor and
do not block evaluation; they belong in `usage.md` alongside the
behaviour they qualify.

### Decision 2 — `docs/` split: two files

- `docs/usage.md` — everything an end user does *after* install:
  CLI reference (one section per subcommand), JSON contract (envelopes
  + error shape + exit codes), embedded-skill details, known
  limitations, manual install procedure.
- `docs/development.md` — contributing (tests, OpenSpec workflow) and
  release procedure.

**Rationale:** matches the two natural audiences (users vs.
contributors). One file per topic was considered but rejected — five
or six tiny files is more friction than one medium file with anchored
sections, given the doc volume.

**Alternative considered:** single `docs/README.md` with TOC.
Rejected — mixes "I just want to use this" with "I want to release a
new version", which is exactly the split we are trying to introduce.

### Decision 3 — Content fidelity

Doc content is copied verbatim from the current README into the new
files. Wording is not rewritten. Anchors/heading levels stay
consistent so external readers who land on the new file via search
recognise the section names.

**Rationale:** a content rewrite expands scope and risks introducing
errors in CLI flag tables / JSON shapes. This change is structural,
not editorial.

## Risks / Trade-offs

- **External deep links break** — anyone who linked to e.g.
  `README.md#json-contract` gets a 404 fragment.
  → Mitigation: the project is v0.x with no published external
  documentation links. Accept the break.
- **Search engines index two pages** — duplicate-ish info if anyone
  ever indexes both. → Mitigation: docs are unique now (no overlap),
  and the README links explicitly to the canonical doc files.
- **Doc drift** — splitting raises the chance README/docs disagree.
  → Mitigation: README contains no reference content, so there is
  nothing to drift. Each fact lives in exactly one place.

## Migration Plan

This is a docs-only change:

1. Create `docs/usage.md` and `docs/development.md` with content
   moved from README sections.
2. Rewrite `README.md` to the lean shape from Decision 1.
3. Verify all internal links: README → `docs/*.md` resolve;
   `docs/*.md` → other repo files (LICENSE, openspec/, scripts/)
   resolve.
4. No code changes, no build/test impact.

Rollback: `git revert` of the single commit restores the old README.
No data or state to migrate.
