#!/usr/bin/env bash
# Bootstrap local OpenSpec workspace.
# The openspec/ directory is gitignored; only specs/ at repo root is tracked.
# This script recreates the OpenSpec layout and symlinks openspec/specs -> ../specs.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

mkdir -p openspec/changes/archive
mkdir -p specs

if [ ! -e openspec/specs ]; then
  ln -s ../specs openspec/specs
fi

if [ ! -f openspec/config.yaml ]; then
  cat > openspec/config.yaml <<'YAML'
schema: spec-driven

# Project context (optional). Shown to AI when creating artifacts.
# context: |
#   Tech stack: Go, ...
YAML
fi

echo "OpenSpec workspace ready at openspec/ (specs -> $(readlink openspec/specs))"
