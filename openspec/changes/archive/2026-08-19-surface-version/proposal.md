## Why

The binary already carries its release version — `main.version`, injected at build time by the release workflow via `-ldflags "-X main.version=${TAG}"`. But nothing on the viewer surface exposes it: the topbar shows the project name and an SSE status dot, and the `ServerStatus` overlay shows only `Status` and `Storage`. A user running `ezida serve` has no way to tell which release they are looking at, and support questions like "which version is this?" require hopping to the terminal and running `ezida --version`. The version is already computed at boot; it just never reaches the UI.

## What Changes

- Add a `version` field to the `GET /api/board` response, mirroring the existing `project_name` field: resolved once at server start from the build-time `main.version` string, immutable for the process lifetime.
- Surface that `version` in the viewer's `ServerStatus` overlay as a new `Version` row, alongside `Status` and `Storage`.
- Expose the same value on the CLI side as `server.Version` (a package-level variable overridable via `-ldflags`, exactly like `main.version`), so the server package owns its copy instead of reaching across `main`.
- `ezida export --json` carries the same `version` field in its envelope, so a static demo snapshot records the binary that produced it.
- Update `docs/development.md`'s "Releasing" section: remove the stale "Bump the version in CHANGELOG.md if/when added (none yet for v0.1.0)" note (there is no CHANGELOG, and the version is injected by the workflow, not edited by hand), and document that the release version is visible via `ezida --version` and in the viewer overlay.

No breaking changes. No new CLI flags. No schema migration.

## Capabilities

### New Capabilities

- `viewer-version-surface`: the viewer server resolves and exposes the build version, and the UI renders it. Covers the `version` field on `GET /api/board`, the `server.Version` seam, and the `Version` row in the `ServerStatus` overlay.

### Modified Capabilities

- `viewer-server`: the `GET /api/board` response gains a new top-level `version` string field, resolved once at boot and immutable thereafter. The existing `project_name` requirement is the template this follows.
- `board-export`: the `ezida export --json` envelope gains a `version` field, so static snapshots record the producing binary.
- `viewer-ui`: the `ServerStatus` overlay gains a `Version` row.

## Impact

- **Go**: `internal/server/server.go` (new `Version` package var + field on `serverState`, wired in `Run` and `handleBoard`), `internal/server/handlers.go` (`boardResponse.Version`), `internal/commands/export.go` (`ExportEnvelope.Version`), `cmd/ezida/main.go` (pass `version` into `server.Version`).
- **React**: `internal/server/web/app.jsx` (`ServerStatus` overlay gains a `Version` row).
- **Build/release**: `.github/workflows/release.yml` already injects `-X main.version=${TAG}`; add the matching `-X github.com/nicolasvergoz/ezida-kanban/internal/server.Version=${TAG}` on the same ldflags line. `ci.yml` and `pages.yml` leave `version` as `dev` — no change needed there.
- **Docs**: `docs/development.md` releasing section.
- **Tests**: new scenarios in `viewer-server`, `viewer-ui`, and `board-export` specs; existing tests assert on response shapes and will need the new field added to fixtures.