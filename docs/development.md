# Development

Contributing and release procedure for `ezida`. For end-user docs,
see [usage.md](./usage.md). For the project pitch and quick start,
see the [README](../README.md).

## Contributing

The project's specs and change history live under
[`openspec/`](../openspec/). Each phase of v1 was developed as an
OpenSpec change with proposal, design, and per-capability spec deltas.

To run the test suite locally:

```sh
go test ./...
go vet ./...
shellcheck -s sh scripts/install.sh
```

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
