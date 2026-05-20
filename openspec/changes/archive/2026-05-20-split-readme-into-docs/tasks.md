## 1. Create docs directory

- [x] 1.1 Create `docs/` at repository root
- [x] 1.2 Write `docs/usage.md` with: CLI reference (one section per
  subcommand: `init`, `board`, `list`, `get`, `add`, `edit`, `move`,
  `rm`, `columns`, `priorities`), JSON contract (envelopes + error
  shape + exit codes), embedded-skill section, known-limitations
  section, manual install section. Content moved verbatim from
  current README sections.
- [x] 1.3 Write `docs/development.md` with: contributing section
  (test commands `go test ./...`, `go vet ./...`, `shellcheck -s sh
  scripts/install.sh`; OpenSpec workflow links) and the full
  release procedure (tag push, workflow watching, smoke test).
  Content moved verbatim from current README sections.

## 2. Rewrite README

- [x] 2.1 Replace `README.md` contents with: title, tagline,
  supported-platforms line, one-liner install (curl + version pin),
  quick start (`ezida init` + a few example commands), prominent
  links to `docs/usage.md` and `docs/development.md`, License
  section.
- [x] 2.2 Verify README is significantly shorter than the original
  355 lines (target: under 100 lines). — 63 lines, under target.

## 3. Verify links and content

- [x] 3.1 `grep` every relative link in the new `README.md` and
  `docs/*.md` files; confirm each target file exists. — all 8 unique
  relative links resolve (`docs/usage.md`, `docs/development.md`,
  `LICENSE`, `openspec/`, `openspec/changes/`, `openspec/specs/`,
  `README.md` from docs, `docs/usage.md#manual-install` anchor).
- [x] 3.2 Diff the union of new README + `docs/usage.md` +
  `docs/development.md` against the old README to confirm no
  content was lost (allow for heading-level shifts and link
  rewrites). — every section of the original README (intro, install
  one-liner, manual install, EZIDA_VERSION pin, less-inspect path,
  quick start, full CLI ref, JSON contract, error envelope, embedded
  skill, known limitations, contributing, releasing, license) is
  present in either README or docs/. Wording is verbatim.
- [x] 3.3 Run `go vet ./...` and `go test ./...` to confirm the
  docs-only change has not affected the Go build (sanity check —
  no Go files were touched, so this should pass). — both pass.
