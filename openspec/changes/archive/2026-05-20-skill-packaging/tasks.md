## 1. Import and patch the skill

- [x] 1.1 Create `internal/skill/SKILL.md` by copying `refs/SKILL.md` verbatim. Done when `wc -l` of the two files is equal.
- [x] 1.2 Apply patch 1: replace `installed via pip` with `installed via the install script` in `internal/skill/SKILL.md`. Done when `grep -c "installed via the install script" internal/skill/SKILL.md` returns `1` and `grep -c "installed via pip" internal/skill/SKILL.md` returns `0`.
- [x] 1.3 Apply patch 2: remove the Python fallback paragraph and the `python <skill-directory>/ezida.py` code block, and adjust the sentence around it so the section flows (no orphan `Otherwise,`). Done when `grep -c "python <skill-directory>" internal/skill/SKILL.md` returns `0` and a manual read of the surrounding paragraph still parses naturally.

## 2. Embed package

- [x] 2.1 Create `internal/skill/skill.go` containing `package skill`, the blank import `_ "embed"`, and `//go:embed SKILL.md` with `var Bytes []byte`. Done when `go build ./internal/skill` exits 0.
- [x] 2.2 Create `internal/skill/skill_test.go` with `TestBytes_MatchesFile` (read `SKILL.md` via `os.ReadFile`, compare to `skill.Bytes`), `TestBytes_NoPythonReferences`, `TestBytes_NoPipReferences`, `TestBytes_MentionsInstallScript`. Done when `go test ./internal/skill` exits 0.
- [x] 2.3 Verify the negative case: rename `internal/skill/SKILL.md` away, run `go build ./...`, confirm it fails with a `//go:embed` error. Restore the file. Done when the negative observation holds.

## 3. Update `ezida init`

- [x] 3.1 Add `--skill-only` boolean flag to `NewInitCmd`. Done when `ezida init --help` lists it.
- [x] 3.2 Add the `writeSkillFile(path string) error` helper in `internal/commands/init.go` that `MkdirAll` the parent with mode `0755` and `os.WriteFile` the embedded bytes with mode `0644`. Done when a unit test confirms a nested missing parent directory is created.
- [x] 3.3 Refactor `runInit` per the design: handle `--skill-only` path early (skip board logic entirely, write skill, render text or JSON envelope); for the full path, write `kanban.toml` first then the skill, then render text (with trailing comment note) or extended JSON envelope. Done when every spec scenario for `ezida init` (P5 additions + P2 original) passes.
- [x] 3.4 Add `TestInit_WritesSkill`, `TestInit_SkillOnly_DoesNotCreateBoard`, `TestInit_SkillOnly_DoesNotTouchExistingBoard`, `TestInit_JSONEnvelope_Full`, `TestInit_JSONEnvelope_SkillOnly`, `TestInit_TextOutput_IncludesCommentNote`, `TestInit_SkillOnly_TextOutput_NoBoardMention`. Done when `go test ./internal/commands -run TestInit` exits 0.

## 4. Documentation seeds

- [x] 4.1 Add a one-line note at the top of `internal/skill/SKILL.md` (inside a HTML comment so it does not render in the AI's view) reminding contributors that this file is the source of truth and edits to `refs/SKILL.md` are NOT propagated automatically. Done when the comment is present and the embedded bytes still pass the P5 negative-reference tests.

## 5. Acceptance gate

- [x] 5.1 Run `go test ./... && go vet ./...` from the repo root. Done when both exit 0.
- [x] 5.2 In a clean temp directory: `ezida init` → confirm `kanban.toml` exists, `.claude/skills/ezida-kanban/SKILL.md` exists, and stdout's last line is the TOML-comment note. Done when the manual observation matches.
- [x] 5.3 In the same directory: edit `kanban.toml` (add a sentinel comment), then run `ezida init --skill-only`, confirm `kanban.toml` is byte-unchanged. Done when `diff` shows zero differences.
- [x] 5.4 Open Claude Code in the project directory and confirm the skill is discovered on the next session (manual). Done when the assistant recognises `ezida-kanban` as an available skill. (If the manual UI step is impractical in the build pipeline, the contributor running P5 records the observation in the PR description.) — deferred to PR contributor per task fallback clause; build-pipeline cannot exercise the Claude Code UI.
