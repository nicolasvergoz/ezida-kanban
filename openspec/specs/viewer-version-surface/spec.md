# viewer-version-surface

## Purpose

The build version, surfaced to the viewer UI. The binary already
carries its release version (`main.version`, injected at build time
via ldflags); this capability is the contract that exposes it to the
viewer and the UI that renders it. The value is resolved once at
server boot and is immutable for the process lifetime — it is a build
constant, not a board value, and it is never re-evaluated.

## Requirements

### Requirement: Server exposes a build-version package variable

The `internal/server` package SHALL export a package-level variable
`Version` of type `string`, defaulting to `"dev"`. It is overridable
at build time via ldflags, exactly like `main.version` in
`cmd/ezida/main.go`. The release workflow injects the release tag;
local builds and tests leave it at `"dev"`.

#### Scenario: Default value for local builds
- **WHEN** the server package is compiled without ldflags override
- **THEN** `server.Version` equals `"dev"`

#### Scenario: Release workflow overrides the value
- **WHEN** the binary is built with `-ldflags "-X <server-package-path>.Version=v0.4.0-beta"`
- **THEN** `server.Version` equals `"v0.4.0-beta"`

### Requirement: `server.Run` injects the version into serverState

`server.Run` (and its testable twin `runWithContext`) SHALL copy the
package `Version` constant into the constructed `serverState` before
wiring routes, so the value is captured once at boot and never
re-read. The test helper `startTestServer` exercises the same path.

#### Scenario: Version is captured at boot
- **WHEN** `runWithContext` is invoked with a serverState whose
  `version` field is unset
- **THEN** after boot the serverState's `version` equals the package
  `server.Version` value

#### Scenario: Version does not change across the process lifetime
- **WHEN** `server.Version` is mutated after the server has started
- **THEN** the value already captured in serverState is unchanged, and
  every subsequent response reflects the original boot-time value

### Requirement: `GET /api/board` carries the version field

`GET /api/board` SHALL include a top-level `version` string field in
its JSON response, set to the boot-time value from `serverState`. The
field is always present, even for local builds (it renders `"dev"`).
It is resolved once at server start and MUST NOT be re-evaluated on
each request — it is not derived from the board file.

#### Scenario: Version field present in response
- **WHEN** `GET /api/board` is called against a running server
- **THEN** the response body contains a top-level string field
  `version`
- **AND** the value equals the string the binary was built with

#### Scenario: Version is stable across requests even when the board changes
- **WHEN** `GET /api/board` is called twice against the same running
  server with the board file rewritten in between
- **THEN** both responses contain the same `version` value

### Requirement: `ezida export --json` carries the version field

`ezida export --json` SHALL include a top-level `version` string field
in its JSON envelope, set to the same `server.Version` value the
viewer would expose. This makes a static snapshot record the binary
that produced it.

#### Scenario: Export carries the same version as the viewer
- **WHEN** `ezida export --json` is run from a project root
- **THEN** the emitted JSON contains a top-level `version` field
- **AND** the value equals `server.Version`

### Requirement: The viewer renders a Version row in the ServerStatus overlay

The `ServerStatus` overlay in the embedded viewer SHALL render a
`Version` row showing the build version, placed after the existing
`Storage` row. The value is the raw `version` string from the board
payload — shown verbatim, with no parsing or reformatting. A local
build renders `"dev"`.

#### Scenario: Version row present in the overlay
- **WHEN** the `ServerStatus` overlay is opened in a running viewer
- **THEN** the overlay contains a row whose label is `Version`
- **AND** its value is the `version` string from the last `/api/board`
  response

#### Scenario: Local build renders "dev"
- **WHEN** the viewer is served by a binary built without ldflags
  override
- **THEN** the `Version` row's value is `"dev"`

#### Scenario: Release build renders the tag
- **WHEN** the viewer is served by a binary built with
  `server.Version=v0.4.0-beta`
- **THEN** the `Version` row's value is `"v0.4.0-beta"`