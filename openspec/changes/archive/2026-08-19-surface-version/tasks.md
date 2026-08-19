## 1. Server seam — expose the build version

- [x] 1.1 Add `Version` package variable to `internal/server/server.go` (default `"dev"`, ldflags-overridable, mirroring `main.version`).
- [x] 1.2 Add `version` field to `serverState` in `internal/server/server.go`.
- [x] 1.3 In `runWithContext`, copy the package `Version` into `serverState` before wiring routes.
- [x] 1.4 In the test helper `startTestServer` (`internal/server/server_test.go`), assign `version` the same way so tests use the real path.
- [x] 1.5 In `cmd/ezida/main.go`, assign `server.Version = version` before calling `server.Run`.

## 2. Wire — carry version on the board payload and the export envelope

- [x] 2.1 Add `Version string` field to `boardResponse` in `internal/server/handlers.go` and set it from `serverState.version` in `handleBoard`.
- [x] 2.2 Add `Version string` field to `ExportEnvelope` in `internal/output/json.go`.
- [x] 2.3 In `runExport` (`internal/commands/export.go`), populate `ExportEnvelope.Version` from `server.Version` (read directly, mirroring the existing `resolveProjectName` duplication).
- [x] 2.4 Pass the version through the `toUiBoard` adapter in `internal/server/web/app.jsx` so the UI receives it unchanged, the same way `project_name` passes through.

## 3. UI — render the Version row in the ServerStatus overlay

- [x] 3.1 Add a `Version` row to the `ServerStatus` overlay in `internal/server/web/app.jsx`, placed after the `Storage` row, reading `board.version`.
- [x] 3.2 Show the value verbatim — no parsing, no reformatting; `dev` renders as `dev`.

## 4. Build — inject both ldflags symbols on the release path

- [x] 4.1 In `.github/workflows/release.yml`, add `-X github.com/nicolasvergoz/ezida-kanban/internal/server.Version=${TAG}` to the same `-ldflags` line that already injects `main.version`.
- [x] 4.2 Confirm `ci.yml` and `pages.yml` leave `server.Version` as `dev` (no change needed; verify by reading them).

## 5. Docs — correct the stale release note

- [x] 5.1 In `docs/development.md`, remove the "Bump the version in CHANGELOG.md if/when added (none yet for v0.1.0)" note.
- [x] 5.2 Document that the release version is injected by the workflow, visible via `ezida --version`, and surfaced in the viewer's ServerStatus overlay.

## 6. Tests — cover the new contract

- [x] 6.1 Add a `version` field to every `GET /api/board` response fixture in `internal/server/server_test.go` and update assertions that check the envelope shape.
- [x] 6.2 Add scenarios for `version` reflecting the build constant, rendering `"dev"` for a local build, and stability across requests.
- [x] 6.3 Add export tests asserting `version` is present and matches `server.Version`.
- [x] 6.4 Add e2e coverage for the `Version` row in the overlay (present, shows the build value).
- [x] 6.5 Run `./scripts/verify.sh` (gofmt, go vet, go test, shellcheck, Playwright) until green.
