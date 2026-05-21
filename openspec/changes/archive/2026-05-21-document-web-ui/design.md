## Context

`ezida serve` and the interactive Web UI it serves shipped over a
series of changes (`viewer-server`, `viewer-ui`, `viewer-hot-reload`,
`redesign-card-detail-modal`, `add-inline-card-create-delete`,
`add-inline-column-ops`, `add-card-move-reorder`, `add-board-filter`,
`add-dark-theme`, `redesign-tokens-and-chrome`). Behaviour is fully
specified at `openspec/specs/viewer-server/spec.md` and
`openspec/specs/viewer-ui/spec.md`, but the user-facing docs
(`README.md`, `docs/usage.md`) still describe `ezida` as a CLI-only
tool. A first-time reader has no way to discover the Web UI.

## Goals / Non-Goals

**Goals:**
- Make the Web UI discoverable from the README quick start — one
  paragraph plus the command (`ezida serve`), positioned right
  after the CLI examples.
- Give `ezida serve` a full reference section in `docs/usage.md`
  with the same shape as other subcommands (one-line summary,
  flag table, example), but extended with a short capabilities
  list so readers know what the page can do.
- Keep both updates faithful to the shipped behaviour: read,
  inline create/edit/delete on cards, drag-reorder cards and
  columns, inline column add/rename/delete, board filter, dark
  theme, and SSE hot reload of `kanban.toml`.

**Non-Goals:**
- No code changes, no flag changes, no JSON contract changes.
- No screenshots or GIFs (out of scope for v1 docs; markdown only).
- No standalone `docs/web-ui.md` file — single-section in
  `usage.md` keeps the doc surface flat, matching the precedent
  set by `split-readme-into-docs`.
- No SEO/marketing copy. Voice stays terse and reference-y,
  matching the existing docs.

## Decisions

- **README mention is a single sub-section, not a feature
  showcase.** A 3–5 line paragraph plus one fenced `ezida serve`
  block — enough to surface the feature without bloating the
  landing page. The CLI examples remain primary because the CLI
  is still the authoritative editing surface (the Web UI sits on
  top of the same commands via the `/api/*` handlers).
- **Position in usage.md: after `ezida priorities`, before the
  JSON contract section.** The Web UI uses the same JSON contract
  internally, so introducing it before that section reads
  naturally. It is also the only subcommand that produces a
  long-running process, which justifies the placement at the end
  of the CLI reference.
- **Frame the page as "Web UI", not "viewer".** The page is now
  fully read-write; calling it a viewer undersells what shipped.
  The command name (`ezida serve`) stays untouched for back-
  compat.

## Risks / Trade-offs

- [Docs drift further from code over time] → Mitigated by linking
  the usage.md section to the two spec files
  (`openspec/specs/viewer-server/spec.md`,
  `openspec/specs/viewer-ui/spec.md`) so future contributors have
  an obvious place to reconcile.
- [README grows back toward pre-split bloat] → Mitigated by
  keeping the Web UI sub-section to a single short paragraph and
  one command. Reference detail stays in usage.md.
- [Calling it "Web UI" while the binary subcommand is `serve` and
  the embedded directory is `web/`] → Acceptable: the command
  name is a verb (what it does), the section name is a noun (what
  it serves). Both are stable.
