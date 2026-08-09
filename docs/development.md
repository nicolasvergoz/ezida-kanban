# Development

Contributing and release procedure for `ezida`. For end-user docs,
see [usage.md](./usage.md). For the project pitch and quick start,
see the [README](../README.md).

## Contributing

The project's specs and change history live under
[`openspec/`](../openspec/). Each phase of v1 was developed as an
OpenSpec change with proposal, design, and per-capability spec deltas.

To run the full verification loop locally:

```sh
./scripts/verify.sh
```

That is gofmt, `go vet`, `go test ./...`, shellcheck, and the browser
tests. Pass `--go` to skip the browser half when you have not set it
up, or `--visual` to also compare the viewer against pixel baselines.

### Browser tests

The viewer is covered by [Playwright](https://playwright.dev) specs in
[`e2e/`](../e2e/). Each one compiles the CLI from the working tree,
boots the real `ezida serve` against a throwaway fixture board, and
drives the page — so a single run covers the Go handlers, the JSON
wire, the adapter, and the rendering.

They need their dependencies once per checkout:

```sh
npm install
npx playwright install chromium
npx playwright test           # or: npm run test:e2e
npx playwright test --ui      # pick and debug individual tests
```

Visual comparisons are opt-in, because their baselines are captured on
one machine's font stack and will not match elsewhere:

```sh
npm run test:e2e:visual
npm run test:e2e:update-snapshots   # after an intended visual change
```

Two things the suite deliberately does not cover: whether the UI
*looks* right, which still needs a human, and native HTML5 drag and
drop, which Playwright's mouse synthesis cannot reliably drive.

CI runs the Go gate only — the browser tests are a local loop for now.

Development uses the OpenSpec workflow. The relevant slash commands
in Claude Code are:

- `/opsx:new` — start a new change.
- `/opsx:propose` — create the change with all artifacts in one step.
- `/opsx:apply` — implement the change's tasks.

See [`openspec/changes/`](../openspec/changes/) for change templates
and the archived history.

## Releasing

The official first-release procedure:

1. Bump the version in `CHANGELOG.md` if/when added (none yet for v0.1.0).
2. From a clean `main` checkout, push the tag: `git tag v0.1.0 && git push origin v0.1.0`.
   The release workflow refuses tags not reachable from `main`.
3. Watch the workflow: `gh run list --workflow=release.yml --limit 1`
   to find the run id, then `gh run watch <run-id>`. It must produce
   four tarballs, `checksums.txt`, and `install.sh` (six assets).
4. Smoke-test the install on a fresh machine:
   `curl -sSL https://github.com/nicolasvergoz/ezida-kanban/releases/latest/download/install.sh | sh`
   and confirm `~/.local/bin/ezida --version` prints `v0.1.0`.
