## ADDED Requirements

### Requirement: Install script — platform detection and download

`scripts/install.sh` SHALL detect the host platform via `uname -sm` and
map the pair to one of the four supported targets:

- `Darwin arm64` → `darwin_arm64`
- `Darwin x86_64` → `darwin_amd64`
- `Linux aarch64` or `Linux arm64` → `linux_arm64`
- `Linux x86_64` → `linux_amd64`

For any other combination, the script MUST exit with status `1` and
print a message naming the detected pair and the four supported
combinations.

The script SHALL resolve the version to install in the following order:

1. The `EZIDA_VERSION` environment variable, when non-empty.
2. The latest GitHub release (queried via the `releases/latest` API
   endpoint and parsed without `jq`).

The download URLs MUST be:

- Tarball: `https://github.com/nicolasvergoz/ezida-kanban/releases/download/<version>/ezida_<version>_<os>_<arch>.tar.gz`
- Checksums: `https://github.com/nicolasvergoz/ezida-kanban/releases/download/<version>/checksums.txt`

#### Scenario: macOS Apple Silicon resolves to darwin_arm64

- **WHEN** the script runs on a host where `uname -sm` outputs
  `Darwin arm64`
- **THEN** the resolved target string MUST equal `darwin_arm64`

#### Scenario: Linux x86 resolves to linux_amd64

- **WHEN** the script runs on a host where `uname -sm` outputs
  `Linux x86_64`
- **THEN** the resolved target string MUST equal `linux_amd64`

#### Scenario: Unsupported platform exits 1

- **WHEN** the script runs on a host where `uname -sm` outputs
  `Linux i686`
- **THEN** the script MUST exit `1`
- **AND** stderr MUST mention `i686` and the four supported platforms

#### Scenario: EZIDA_VERSION override

- **WHEN** the script is invoked with `EZIDA_VERSION=v0.2.0` set
- **THEN** the download URL MUST embed `v0.2.0` (not the latest tag)

### Requirement: Install script — checksum verification

The script SHALL download `checksums.txt` alongside the chosen tarball
and verify the tarball's SHA256 matches the entry in `checksums.txt`
before extracting. On mismatch, the script MUST exit `1` and MUST NOT
leave a half-installed binary on disk.

The verification MUST use whichever of `sha256sum` / `shasum -a 256`
is available on the host. The script MUST detect this once at startup
and exit `1` with a clear message if neither tool is available.

#### Scenario: Matching checksum installs the binary

- **WHEN** the downloaded tarball's SHA256 matches its `checksums.txt`
  entry
- **THEN** the binary is extracted and copied to `~/.local/bin/ezida`
- **AND** the script exits `0`

#### Scenario: Mismatched checksum aborts cleanly

- **WHEN** the downloaded tarball is corrupted (SHA256 does not match)
- **THEN** the script exits `1`
- **AND** stderr contains `Checksum verification failed`
- **AND** the binary at `~/.local/bin/ezida` is unchanged (or absent
  if it was not present before)

#### Scenario: No sha256 tool available

- **WHEN** the script runs on a host where neither `sha256sum` nor
  `shasum` is on `PATH`
- **THEN** the script exits `1` before any download begins
- **AND** stderr names both tools as required

### Requirement: Install script — install location and PATH reminder

The script SHALL install the binary at `~/.local/bin/ezida` with
permissions `0755`. It MUST create `~/.local/bin/` (mode `0755`) if
missing. The previous file at that path, if any, MUST be overwritten
silently.

After a successful install, if `~/.local/bin` does NOT appear in the
host's `PATH` (`echo ":$PATH:" | grep -F ":$HOME/.local/bin:"` fails),
the script MUST print a single reminder line on stdout:
```
note: add $HOME/.local/bin to your PATH
```
The script MUST NOT modify any shell configuration files
(`~/.bashrc`, `~/.zshrc`, `~/.profile`, etc.).

#### Scenario: Fresh install creates the directory

- **WHEN** the script runs on a host where `~/.local/bin/` does not
  exist
- **THEN** the directory exists after the run
- **AND** the binary is present inside it with mode `0755`

#### Scenario: Existing binary is replaced silently

- **WHEN** the script runs on a host where `~/.local/bin/ezida` already
  exists
- **THEN** the file is replaced without prompting
- **AND** stdout does NOT mention overwrite

#### Scenario: PATH reminder appears when needed

- **WHEN** the script runs on a host where `$PATH` does not include
  `$HOME/.local/bin`
- **THEN** stdout contains a line equal to
  `note: add $HOME/.local/bin to your PATH`

#### Scenario: No reminder when PATH is fine

- **WHEN** the script runs on a host where `$PATH` already includes
  `$HOME/.local/bin`
- **THEN** stdout does NOT contain the reminder line

### Requirement: Release workflow — trigger and gate

A GitHub Actions workflow at `.github/workflows/release.yml` SHALL
trigger on any push of a tag matching the glob `v*`. Before producing
artifacts, the workflow MUST run `go test ./...` and `go vet ./...`.
A failure of either step MUST abort the run before any release is
created.

The workflow MUST refuse to publish a release whose tag was not
created from the `main` branch. (Implementation: the workflow's first
job inspects `github.event.base_ref` and exits non-zero if it does not
equal `refs/heads/main`.)

#### Scenario: Tag on main triggers a successful build

- **WHEN** a `v0.1.0` tag is pushed pointing at a commit on `main`
  whose tests all pass
- **THEN** the workflow completes successfully
- **AND** a GitHub Release exists for `v0.1.0`

#### Scenario: Tag on a non-main branch aborts the workflow

- **WHEN** a `v0.1.1` tag is pushed pointing at a commit that is not
  reachable from `main`
- **THEN** the workflow exits with failure before publishing
- **AND** no GitHub Release is created

#### Scenario: Failing tests abort the workflow

- **WHEN** `go test ./...` exits non-zero on the tagged commit
- **THEN** the workflow exits with failure
- **AND** no GitHub Release is created

### Requirement: Release workflow — cross-compile and package

For each of the four targets (`darwin/arm64`, `darwin/amd64`,
`linux/arm64`, `linux/amd64`), the workflow SHALL:

- Build the binary as `ezida` using
  `GOOS=<os> GOARCH=<arch> go build -trimpath -ldflags "-X main.version=<TAG> -s -w" -o ezida ./cmd/ezida`.
- Package the binary plus the repository's `LICENSE` file into a
  gzipped tarball named `ezida_<TAG>_<os>_<arch>.tar.gz`.

After all four packages exist, the workflow SHALL produce a
`checksums.txt` file containing the SHA256 of every tarball (one line
per tarball, `sha256sum`-compatible format: `<hex>  <filename>`).

#### Scenario: All four artifacts are produced

- **WHEN** a release run completes successfully for tag `v0.1.0`
- **THEN** the GitHub Release exposes exactly these files:
  `ezida_v0.1.0_darwin_arm64.tar.gz`, `ezida_v0.1.0_darwin_amd64.tar.gz`,
  `ezida_v0.1.0_linux_arm64.tar.gz`, `ezida_v0.1.0_linux_amd64.tar.gz`,
  `checksums.txt`, `install.sh`

#### Scenario: Checksums file is well-formed

- **WHEN** `checksums.txt` is downloaded and inspected
- **THEN** it MUST contain exactly four lines
- **AND** each line MUST match the pattern `[0-9a-f]{64}  ezida_<TAG>_.+\.tar\.gz`

#### Scenario: Binary embeds the version

- **WHEN** a tagged binary is extracted and run as `ezida --version`
- **THEN** stdout MUST contain the tag string (e.g. `v0.1.0`)

### Requirement: CI workflow — quality gates on every push and PR

A GitHub Actions workflow at `.github/workflows/ci.yml` SHALL trigger
on every push to `main` and every pull request targeting `main`. It
MUST run all four of the following steps; any failure fails the
workflow:

- `go test ./...`
- `go vet ./...`
- `gofmt -d $(find . -name '*.go' -not -path './vendor/*')` exits with
  zero output (no formatting diffs).
- `shellcheck scripts/install.sh` exits 0.

#### Scenario: Clean PR passes CI

- **WHEN** a PR is opened with formatted code and passing tests
- **THEN** all four CI steps complete successfully

#### Scenario: Formatting drift fails CI

- **WHEN** a PR contains a Go file with unformatted whitespace
- **THEN** the `gofmt -d` step exits non-zero
- **AND** the workflow reports the offending diff in its log

#### Scenario: Shell error in install.sh fails CI

- **WHEN** `scripts/install.sh` is modified with a shellcheck-flagged
  issue
- **THEN** the `shellcheck` step fails

### Requirement: README documents install, usage, and JSON contract

The repository SHALL contain a top-level `README.md` covering, in
order: install instructions (the `curl … | sh` one-liner plus a manual
download path), a quick-start example reproducing brief §8, a CLI
reference enumerating every command and its flags with one-line
descriptions, an overview of the JSON contract (with example envelopes
for `board`, `list`, `get`, and the error shape), a description of
the embedded skill (what it does, how `ezida init` installs it, how
to refresh it with `--skill-only`), and a "known limitations" section
mentioning the four items from the brief (TOML comments not
preserved, no Windows, single board per repo, no real-time
collaboration).

#### Scenario: README enumerates every CLI command

- **WHEN** `README.md` is grep'd for the CLI command names from brief
  §6 (`init`, `board`, `list`, `get`, `add`, `edit`, `move`, `rm`,
  `columns`, `priorities`)
- **THEN** every command name MUST appear in the README at least once

#### Scenario: README documents the JSON contract

- **WHEN** `README.md` is read end-to-end
- **THEN** it MUST contain example JSON envelopes for at least
  `ezida board --json`, `ezida list --json`, `ezida get --json`, and
  the error envelope

### Requirement: LICENSE is MIT

The repository SHALL contain a top-level `LICENSE` file containing the
MIT License with copyright held by the repo owner.

#### Scenario: LICENSE is MIT

- **WHEN** the first line of `LICENSE` is read
- **THEN** it MUST equal `MIT License`
- **AND** the file MUST contain the standard MIT permission grant
  language
