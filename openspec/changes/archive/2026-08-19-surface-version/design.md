## Context

`main.version` in `cmd/ezida/main.go` is the single source of the build
version. It is injected by the release workflow via
`-ldflags "-X main.version=${TAG}"`, and `ezida --version` prints it. The
viewer, however, never sees it: `serverState` already carries
`projectName`, resolved once at boot from the board path's parent
directory, and the `ServerStatus` overlay renders `Status` and `Storage`
rows from it. The version is available at the same boot moment and is
equally immutable — it just never got a field.

Stakeholders: users running `ezida serve` who want to confirm which
release they are on without leaving the browser; the release workflow,
which already injects the value and needs no new plumbing; support,
who can point at a visible version instead of asking the user to run a
command.

Constraints: the version must not drift across the process lifetime, it
must not require a network call, and it must flow through the existing
`GET /api/board` payload rather than a new endpoint.

## Goals / Non-Goals

**Goals:**

- Surface the build version in the viewer with zero user action.
- Reuse the existing `project_name` boot-time pattern: resolve once,
  immutable, carried on the same payload.
- Keep the change additive: new field, new overlay row, new package
  variable. No behavior change for any existing field.

**Non-Goals:**

- No new CLI flags (`--version` already exists and is unaffected).
- No schema migration; `kanban.toml` untouched.
- No per-request re-resolution of the version (it is a build constant,
  not a board value).
- No CHANGELOG file; the stale note in `docs/development.md` is
  corrected instead.

## Decisions

### D1. Server owns its version via a package variable, mirroring `main.version`

`internal/server/server.go` gains:

```go
// Version is the build-time version string, injected via ldflags the
// same way as main.version in cmd/ezida/main.go. Defaults to "dev"
// for local builds and tests; the release workflow overrides it.
var Version = "dev"
```

`cmd/ezida/main.go` assigns `server.Version = version` before calling
`server.Run`, so `main` stays the single place that reads the ldflags
symbol and the server package owns the value it exposes. This mirrors
the existing `main.version` seam rather than importing `main` (which
would create an import cycle).

Alternative considered: have `server.Run` read `main.version` directly.
Rejected — `internal/server` must not depend on `cmd`, the package
boundary that the `resolveProjectName` duplication in
`internal/commands/export.go` exists to preserve.

### D2. `version` rides on `GET /api/board`, next to `project_name`

`boardResponse` gains a `Version string` field with the same
`json:"version"` tag as `project_name`. It is set on `serverState` once
at boot (`s.version = Version`) and read by `handleBoard`. This is the
same pattern `projectName` already uses, including the "immutable for
the lifetime of the process" contract — fsnotify events do not
re-evaluate it.

Alternative considered: a dedicated `GET /api/version` endpoint.
Rejected — the version is a server constant, not a per-request value,
and adding a second endpoint doubles the client's connection surface
for no information gain. The board payload is already fetched on every
load and on every SSE-driven refetch.

### D3. `serverState` carries the value; `Run` injects it

`serverState` gains a `version string` field. `runWithContext`
assigns it from the package `Version` constant right after constructing
`s`, before `routes` is wired. The test helper `startTestServer`
assigns it the same way (`Version`, i.e. `"dev"`), so tests exercise
the real code path without special-casing.

### D4. `ezida export --json` carries the same field

`ExportEnvelope` in `internal/output/json.go` gains `Version string`,
populated from `server.Version` by `runExport`. The demo snapshot in
`site/demo/board.json` therefore records the binary that produced it,
which is the same information the viewer now shows in the overlay.

`runExport` already duplicates `resolveProjectName` rather than
importing `server`; it reads `server.Version` directly the same way —
one line, no new dependency edge.

### D5. UI renders a `Version` row in the `ServerStatus` overlay

`ServerStatus` in `app.jsx` (lines 727-740) gains a third `server-row`:

```
Version    v0.4.0-beta
```

It reads `board.version` from the already-fetched `toUiBoard` adapter,
which passes the field straight through (no resolution, no index
building — the adapter already passes `project_name` through untouched).
The row is placed after `Storage` so the overlay reads
Status → Storage → Version, matching the order of information density.

The value is shown verbatim (the raw `main.version` string, e.g.
`v0.4.0-beta` or `dev`). No parsing, no semantic-version formatting —
the build injects a tag and the UI shows the tag. `dev` is the honest
label for a local build and needs no special casing.

## Risks / Trade-offs

[Risk: tests that assert on the exact `GET /api/board` JSON shape]
→ Mitigation: every fixture and assertion gains a `version` field. The
field is always present, so this is additive, not a shape change.
Tests are updated in the same change.

[Risk: `dev` showing in the overlay for local builds]
→ Mitigation: this is the correct behavior — a local build is not a
release. The release workflow injects the real tag, so deployed
viewers show the real version. No attempt to hide or prettify it.

[Risk: `server.Version` and `main.version` could drift if the release
workflow forgets to inject the new `-X`]→ Mitigation: both are on the
same `-ldflags` line in `.github/workflows/release.yml`, adjacent.
`ci.yml` and `pages.yml` deliberately leave both as `dev`.

[Risk: the demo snapshot (`site/demo/board.json`) now carries a
`version` field that the Pages workflow regenerates] → Mitigation: the
field is populated from `server.Version`, which is `dev` in the
`pages.yml` build. The demo banner already shows a snapshot SHA; the
version row simply makes the build provenance visible.

## Migration Plan

No migration. The change is additive at every layer:

1. Build with the new `server.Version` seam — local builds show
   `dev`, no behavior change.
2. Tag a release; the workflow injects both ldflags symbols; deployed
   binaries and the Pages demo show the real tag.
3. Rollback is a tag re-push — the field is always present, so older
   binaries never break a newer UI, and newer binaries never break an
   older UI (an unknown field is ignored by every consumer here).

## Open Questions

None. The value to surface, the seam to use, the payload to carry it
on, and the UI location are all settled by existing patterns
(`project_name`). The only remaining decision was whether to add a
dedicated endpoint, and that is rejected in D2.