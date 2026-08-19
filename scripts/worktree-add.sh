#!/usr/bin/env bash
# Wrapper around `git worktree add` that keeps agent-context folders in
# sync across worktrees instead of leaving them empty in every new one.
#
#   ./scripts/worktree-add.sh <path> [<commit-ish>]
#   ./scripts/worktree-add.sh -b <branch> <path> [<start-point>]
#
# All arguments are passed straight through to `git worktree add` — this
# script does not reinterpret its syntax.
#
# On first run it also points core.hooksPath at scripts/hooks (see
# scripts/hooks/post-checkout), so plain `git worktree add` — run
# directly, without this wrapper — keeps working automatically too. If
# core.hooksPath is already set to something else, it's left alone and a
# note is printed instead of silently overriding your other hooks.
#
# See scripts/lib/worktree-shared.sh for what actually gets shared and
# why (short version: gitignored config under .claude/.opencode/.refs,
# moved once into ../.worktree-shared next to the repo and symlinked back
# everywhere, including the main worktree).
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/worktree-shared.sh
source "$script_dir/lib/worktree-shared.sh"

say() { printf '\n\033[1m→ %s\033[0m\n' "$1"; }

main_worktree="$(wtshared::main_worktree)"
if [ -z "$main_worktree" ]; then
  echo "worktree-add.sh: not inside a git repository" >&2
  exit 1
fi

hooks_path="$(git -C "$main_worktree" config --get core.hooksPath || true)"
if [ -z "$hooks_path" ]; then
  git -C "$main_worktree" config core.hooksPath "$main_worktree/scripts/hooks"
  say "configured core.hooksPath -> scripts/hooks (plain 'git worktree add' will auto-link from now on)"
elif [ "$hooks_path" != "$main_worktree/scripts/hooks" ]; then
  echo "worktree-add.sh: core.hooksPath is already set to '$hooks_path', leaving it alone — plain 'git worktree add' won't auto-link unless scripts/hooks/post-checkout runs from there too" >&2
fi

say "bootstrap shared store ($(dirname "$main_worktree")/.worktree-shared)"
wtshared::bootstrap

say "git worktree add $*"
before="$(git worktree list --porcelain | awk '/^worktree/{print $2}' | sort)"
git worktree add "$@"
after="$(git worktree list --porcelain | awk '/^worktree/{print $2}' | sort)"
new_worktree="$(comm -13 <(printf '%s\n' "$before") <(printf '%s\n' "$after"))"

if [ -z "$new_worktree" ]; then
  echo "worktree-add.sh: could not determine the new worktree path, skipping symlinks" >&2
  exit 0
fi

# The post-checkout hook (if configured above) already linked these as
# part of `git worktree add`'s own checkout — this is a no-op fallback
# for when it isn't (e.g. core.hooksPath pointed elsewhere).
say "link shared items into $new_worktree"
wtshared::link "$new_worktree"

say "done"
