## Why

P1–P5 built and tested the binary; P6 ships it. Users need a single
`curl … | sh` to land a working `ezida` on their machine. CI needs a
push-a-tag workflow that cross-compiles, generates checksums, and
attaches everything to a GitHub release. The README needs to teach the
two audiences (developer and AI assistant) how to use the tool. The
LICENSE needs to be MIT (brief §9).

## What Changes

- Add `scripts/install.sh`:
  - Detects OS and arch from `uname -sm`, maps to one of the four
    supported tarballs (`darwin/arm64`, `darwin/amd64`, `linux/arm64`,
    `linux/amd64`). Refuses with a clear message on any other platform.
  - Resolves the latest release version by querying the GitHub API
    (no jq dependency — uses `grep` / `sed`). Allows
    `EZIDA_VERSION=v0.1.0` env override.
  - Downloads `ezida_<version>_<os>_<arch>.tar.gz` and `checksums.txt`
    from the matching release.
  - Verifies the tarball's SHA256 against `checksums.txt`. Aborts on
    mismatch with a non-zero exit and a `Checksum verification failed`
    message.
  - Extracts the binary, installs it as `~/.local/bin/ezida` with mode
    `0755`. Overwrites silently.
  - Prints a one-line reminder if `~/.local/bin` is not in `PATH`.
  - Uses `set -euo pipefail` and avoids bashisms beyond what is
    portable to `/bin/sh` on macOS. (POSIX shell — confirmed by
    running `shellcheck` cleanly.)
- Add `.github/workflows/release.yml`:
  - Triggers on tags matching `v*`.
  - Sets up Go via `actions/setup-go@v5` (Go 1.22).
  - Runs `go test ./...` as the gate.
  - Cross-compiles the four targets with
    `GOOS=<os> GOARCH=<arch> go build -ldflags "-X main.version=<TAG> -s -w" -o ezida ./cmd/ezida`.
  - Packages each as `ezida_<TAG>_<os>_<arch>.tar.gz` containing the
    binary plus the LICENSE.
  - Generates `checksums.txt` with `sha256sum` of every tarball.
  - Creates a GitHub Release via `softprops/action-gh-release@v2`
    attaching the four tarballs, the checksums file, and `install.sh`.
  - Refuses to run on any branch other than `main` (validated via the
    workflow's tag-source check).
- Add `README.md` at the repo root covering: install instructions
  (`curl|sh` one-liner + manual download), quick start (the brief §8
  examples), CLI reference (one section per command, table of flags),
  JSON contract overview (link to the per-command spec scenarios),
  skill explanation (how the AI uses it), known limitations
  (TOML comments not preserved, no Windows, single board per repo),
  contribution guidelines (point at OpenSpec workflow).
- Add `LICENSE` (MIT).
- Add `.github/workflows/ci.yml`:
  - Triggers on every push and PR to `main`.
  - Runs `go test ./...`, `go vet ./...`, `gofmt -d` (zero-diff
    check), and `shellcheck scripts/install.sh`.
- Add `.gitignore` entries for the local build artifact (`ezida`,
  `dist/`, `*.tar.gz`) and the temporary skill cache the OS may create.

## Capabilities

### New Capabilities
- `distribution`: the install script behavior, the release workflow,
  the CI workflow, the README structure, and the LICENSE.

### Modified Capabilities
None.

## Impact

- New files: `scripts/install.sh`, `.github/workflows/release.yml`,
  `.github/workflows/ci.yml`, `README.md`, `LICENSE`, `.gitignore`
  (already present from scaffolding — extended).
- No new Go code, no new Go dependencies.
- Adds a GitHub-side dependency on
  `actions/checkout`, `actions/setup-go`, `softprops/action-gh-release`.
  Each is pinned to a major version per repository convention.
- The first release tag (`v0.1.0`) is cut by the user after merging
  P6 (out of OpenSpec scope; documented in the README).
- After this phase the project is publicly installable and
  reproducibly built from source.
