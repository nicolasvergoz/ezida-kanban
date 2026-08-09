#!/bin/sh
# Full verification loop: everything that has to be green before a
# change is called done. Run from the repo root.
#
#   ./scripts/verify.sh            Go gate + browser tests
#   ./scripts/verify.sh --go       Go gate only (no browser needed)
#   ./scripts/verify.sh --visual   also compare against pixel baselines
#
# The browser tests compile the CLI from the working tree and drive the
# real `ezida serve`, so they cover the Go handlers, the JSON wire, the
# adapter, and the rendering in one pass.
set -eu

only_go=0
visual=0
for arg in "$@"; do
  case "$arg" in
    --go) only_go=1 ;;
    --visual) visual=1 ;;
    *) echo "unknown flag: $arg" >&2; exit 2 ;;
  esac
done

say() { printf '\n\033[1m→ %s\033[0m\n' "$1"; }

say "gofmt"
drift="$(gofmt -l . | grep -v '^vendor/' || true)"
if [ -n "$drift" ]; then
  echo "gofmt drift:" >&2
  printf '%s\n' "$drift" >&2
  exit 1
fi

say "go vet"
go vet ./...

say "go test"
go test ./...

if [ -f scripts/install.sh ] && command -v shellcheck >/dev/null 2>&1; then
  say "shellcheck"
  shellcheck -s sh scripts/install.sh
fi

if [ "$only_go" -eq 1 ]; then
  say "done (Go only)"
  exit 0
fi

if [ ! -d node_modules/@playwright ]; then
  echo "browser tests need their dependencies:" >&2
  echo "  npm install && npx playwright install chromium" >&2
  echo "or skip them with: ./scripts/verify.sh --go" >&2
  exit 1
fi

say "playwright"
if [ "$visual" -eq 1 ]; then
  PW_VISUAL=1 npx playwright test
else
  npx playwright test
fi

say "done"
