## Context

P6 is the shipping phase. The binary itself is unchanged from P5 — only
the build, the install path, and the public-facing docs are new. The
risks are operational: a broken `install.sh` strands users; a broken
release workflow produces no artifacts; a missing checksum makes
supply-chain attacks trivial. The spec scenarios are written to make
each of those failure modes detectable.

Cross-cutting choices (GitHub repo path, version scheme, naming,
checksums, no signing for v1, install location) come from ADR
`0001-kanban-v1-batch.md` (decisions D17–D21). This design covers the
P6-specific implementation: the install script's portability profile,
the release workflow's job graph, the CI workflow's gate set, and the
README structure.

## Goals / Non-Goals

**Goals:**
- A working one-liner install for any user on the four supported
  platforms.
- A push-a-tag release flow that produces four tarballs + checksums
  + install.sh attached to a GitHub Release.
- A CI workflow that catches obvious regressions on every push and PR
  before they reach `main`.
- A README that lets a stranger install, use, and contribute without
  reading the brief.

**Non-Goals:**
- No GoReleaser dependency. The release flow is small enough to write
  in plain `bash` inside the workflow.
- No Homebrew tap, no apt repo, no Snap. Discoverability comes from
  GitHub Releases.
- No code signing, no notarization (ADR §D19).
- No release-candidate / pre-release publishing flow. Tags are
  semver-final.
- No Windows / 32-bit builds.

## Decisions

### Install script — portability profile

`scripts/install.sh` is written for POSIX shell (`#!/bin/sh`) and
verified with `shellcheck -s sh`. Constraints:

- No bashisms: no `[[ ]]`, no `==` in test, no arrays, no `local`
  outside POSIX-allowed contexts.
- Uses `printf` instead of `echo -e`.
- Uses `command -v <tool>` for tool availability checks.
- Reads the latest version with `curl -fsSL` to the GitHub Releases
  API endpoint, then `grep` + `sed` to extract the `tag_name` field
  (no `jq` dependency).
- SHA256 helper auto-detects the tool:
  ```sh
  if command -v sha256sum >/dev/null 2>&1; then
      compute_sha() { sha256sum "$1" | awk '{print $1}'; }
  elif command -v shasum >/dev/null 2>&1; then
      compute_sha() { shasum -a 256 "$1" | awk '{print $1}'; }
  else
      die "need sha256sum or shasum -a 256"
  fi
  ```
- Downloads to a temp directory created with `mktemp -d`, cleaned via
  `trap "rm -rf '$TMPDIR'" EXIT INT TERM`.
- Verifies checksum **before** extracting. If the verify fails, the
  trap cleans up and the user's existing binary (if any) is untouched.

The full script will be ~80 lines. The spec scenarios drive the test
cases.

### Release workflow — job graph

```
release.yml
└── job: release
    ├── checkout (actions/checkout@v4, fetch-depth: 0)
    ├── verify-main (custom shell step — fails if tag is not on main)
    ├── setup-go (actions/setup-go@v5, go-version: '1.22')
    ├── test (go test ./...)
    ├── vet (go vet ./...)
    ├── build-and-package (matrix of 4: os×arch — runs in parallel)
    ├── checksums (after all matrix entries — sha256sum *.tar.gz > checksums.txt)
    └── publish (softprops/action-gh-release@v2 with all files)
```

The matrix uses the workflow's own `strategy.matrix.target`:

```yaml
matrix:
  target:
    - {os: darwin,  arch: arm64}
    - {os: darwin,  arch: amd64}
    - {os: linux,   arch: arm64}
    - {os: linux,   arch: amd64}
```

Each matrix entry builds in `${{ runner.temp }}/build/<os>-<arch>/` and
uploads the resulting tarball as a workflow artifact. The `checksums`
job downloads all artifacts, generates `checksums.txt`, re-uploads it
plus the tarballs and `scripts/install.sh` to the release.

The `verify-main` step:

```sh
git fetch origin main
if ! git merge-base --is-ancestor "${GITHUB_SHA}" "origin/main"; then
    if ! git merge-base --is-ancestor "origin/main" "${GITHUB_SHA}"; then
        echo "tag commit is not reachable from main" >&2
        exit 1
    fi
fi
```

(Tag commits are typically *on* `main`, not ancestors; the check
confirms either is an ancestor of the other.)

### CI workflow — gate set

```
ci.yml
└── job: gate
    ├── checkout
    ├── setup-go (1.22)
    ├── go test ./...
    ├── go vet ./...
    ├── gofmt check (custom step: fail if `gofmt -d` outputs anything)
    └── shellcheck scripts/install.sh
```

`shellcheck` is installed via `apt-get install -y shellcheck` on the
default `ubuntu-latest` runner.

### README structure

```
# ezida-kanban

> File-based Kanban for software projects. One binary, one TOML file,
> no server, no database.

## Install
  - one-liner: curl -sSL .../install.sh | sh
  - manual: download the tarball, extract, copy to PATH

## Quick start
  - ezida init
  - ezida add "First card" --column=todo
  - ezida list

## CLI reference
  - one section per command (init, board, list, get, add, edit, move,
    rm, columns add|rename|rm, priorities add|rename|rm)
  - each section: usage line + flag table + one example

## JSON contract
  - example envelope for board, list, get, mutating commands
  - error envelope
  - link to the spec scenarios in openspec/specs/

## The embedded skill
  - what it is, how `ezida init` installs it
  - how AI assistants discover it
  - how to refresh: ezida init --skill-only

## Known limitations
  - TOML comments are not preserved across writes
  - No Windows support (Darwin + Linux on amd64/arm64 only)
  - Single board per repo
  - No real-time collaboration

## Contributing
  - install Go 1.22+
  - `go test ./...` to run the suite
  - link to openspec/changes/ and the workflow (`/opsx:new`)
```

The README is committed in P6 with placeholder values where dynamic
data is expected (e.g. the install URL embeds the canonical repo path
per ADR §D17).

### LICENSE

Copy-paste of the MIT License template, copyright holder
`Nicolas Vergoz`, year `2026`.

### `.gitignore` additions

```
# build artifact
/ezida
/dist/
*.tar.gz
checksums.txt

# OS noise
.DS_Store
```

(Existing entries from the initial scaffold are kept.)

## Risks / Trade-offs

- **No GoReleaser** → we own the shell glue. Trade-off: more lines in
  the workflow, fewer transitive dependencies. The shell glue is
  simple enough that the cost is acceptable; the benefit is one less
  thing to upgrade.
- **API-based version lookup** → the install script makes an HTTP
  request per run. If GitHub API rate-limits the unauthenticated
  caller, the install fails. Mitigation: `EZIDA_VERSION` override
  documented in the README.
- **No code signing** → ADR §D19. The checksums + HTTPS-only downloads
  are the only integrity controls. Users running `curl|sh` should
  inspect the script first (the README repeats this).
- **`verify-main` check** → reasonable best-effort, not bulletproof
  (someone with write access to `main` can always force a release).
  Acceptable for v1.
- **POSIX shell only** → harder to write than bash. Worth it because
  macOS's `/bin/sh` differs from `bash`; sticking to POSIX guarantees
  the script runs out of the box.

## Migration Plan

Not applicable — this phase introduces the distribution channel.
Future binary updates re-run the install script (idempotent).

## Open Questions

None.
