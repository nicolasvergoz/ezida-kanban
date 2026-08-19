#!/usr/bin/env bash
# Shared logic for keeping gitignored agent-context folders (SHARED_ROOTS)
# in sync across worktrees, backed by a store at ../.worktree-shared next
# to the repo. Sourced by scripts/worktree-add.sh (manual entry point) and
# scripts/hooks/post-checkout (automatic entry point on `git worktree add`).
#
# Some roots (.claude/) mix tracked files (e.g. a skill shipped with the
# repo) with ignored local config, so the whole directory can't be a
# symlink — only its individually-ignored children can be. `git status
# --ignored` gives exactly that granularity: fully-ignored directories
# collapse to one line, tracked subtrees are skipped entirely.

WTSHARED_ROOTS=(.claude .opencode .refs)

wtshared::main_worktree() {
  git worktree list --porcelain | awk '/^worktree/{print $2; exit}'
}

wtshared::shared_dir() {
  dirname "$(wtshared::main_worktree)"
  # caller appends /.worktree-shared
}

# Echoes the currently-gitignored paths under WTSHARED_ROOTS, one per
# line, relative to the main worktree (e.g. ".claude/commands", ".refs").
wtshared::discover() {
  local main_worktree
  main_worktree="$(wtshared::main_worktree)"
  git -C "$main_worktree" status --ignored --porcelain -- "${WTSHARED_ROOTS[@]}" |
    sed -n 's:^!! ::p' |
    sed 's:/$::' |
    grep -v '/\.DS_Store$' |
    grep -v '^\.DS_Store$' || true
}

# Idempotent: moves each discovered item's real content (first time only)
# into the shared store, then makes the main worktree's copy a symlink to
# it. Safe to call repeatedly and from either entry point.
wtshared::bootstrap() {
  local main_worktree shared_dir item src main_item
  main_worktree="$(wtshared::main_worktree)"
  shared_dir="$(dirname "$main_worktree")/.worktree-shared"

  while IFS= read -r item; do
    [ -n "$item" ] || continue
    src="$shared_dir/$item"
    main_item="$main_worktree/$item"

    if [ ! -e "$src" ] && [ ! -L "$src" ]; then
      mkdir -p "$(dirname "$src")"
      if [ -e "$main_item" ] && [ ! -L "$main_item" ]; then
        mv "$main_item" "$src"
      else
        mkdir -p "$src"
      fi
    fi

    if [ -L "$main_item" ]; then
      : # already linked from a previous run
    elif [ -e "$main_item" ]; then
      echo "worktree-shared: $main_item exists and is not a symlink, leaving it alone" >&2
    else
      mkdir -p "$(dirname "$main_item")"
      ln -s "$src" "$main_item"
    fi
  done < <(wtshared::discover)
}

# Symlinks every discovered item into $1 (a worktree directory), skipping
# anything already present there.
wtshared::link() {
  local target="$1" main_worktree shared_dir item dest
  main_worktree="$(wtshared::main_worktree)"
  shared_dir="$(dirname "$main_worktree")/.worktree-shared"

  while IFS= read -r item; do
    [ -n "$item" ] || continue
    dest="$target/$item"
    if [ -e "$dest" ] || [ -L "$dest" ]; then
      echo "worktree-shared: $dest already exists, skipping" >&2
      continue
    fi
    mkdir -p "$(dirname "$dest")"
    ln -s "$shared_dir/$item" "$dest"
    echo "  $item -> $shared_dir/$item"
  done < <(wtshared::discover)
}
