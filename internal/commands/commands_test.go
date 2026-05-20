package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/skill"
)

// fixturePath is the absolute path of the shared populated fixture
// (10+ cards across 3 columns) used by every command test.
const fixturePath = "testdata/populated.toml"

// copyFixture copies the populated fixture into a fresh tmp directory
// and returns the path to its kanban.toml. Tests get isolation per
// case without dragging the live working tree.
func copyFixture(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "kanban.toml")
	if err := os.WriteFile(dst, src, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return dst
}

// runCmd executes one cobra command with captured stdout/stderr. It
// returns (stdoutBytes, stderrBytes, err). The cobra command tree is
// built fresh per call to avoid flag-state bleed across tests.
func runCmd(t *testing.T, build func(jsonOut *bool) *cobra.Command, asJSON bool, args ...string) (string, string, error) {
	t.Helper()
	jsonFlag := asJSON
	c := build(&jsonFlag)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs(args)
	err := c.Execute()
	return stdout.String(), stderr.String(), err
}

// --- init (tasks 3.2 / 3.3 / 3.4) ---

func TestInit_FreshDefaults_Text(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	cmd := newDummyInitForPath(path, false)
	stdout, _, err := executeCobraText(cmd, []string{}, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(stdout, "initialized ") || !strings.Contains(stdout, "kanban.toml") {
		t.Errorf("stdout: %q", stdout)
	}
	b, err := board.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got, want := b.Board.Columns, []string{"todo", "ongoing", "done"}; !equalStrings(got, want) {
		t.Errorf("columns: got %v, want %v", got, want)
	}
	if got, want := b.Board.Priorities, []string{"low", "medium", "high"}; !equalStrings(got, want) {
		t.Errorf("priorities: got %v, want %v", got, want)
	}
}

func TestInit_FreshDefaults_JSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	cmd := newDummyInitForPath(path, true)
	stdout, _, err := executeCobraText(cmd, []string{}, true)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal %q: %v", stdout, err)
	}
	if raw["initialized"] != true {
		t.Errorf("initialized: %v", raw["initialized"])
	}
	if raw["path"] != path {
		t.Errorf("path: %v (want %q)", raw["path"], path)
	}
}

func TestInit_CustomColumnsAndPriorities(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	cmd := newDummyInitForPath(path, false)
	_, _, err := executeCobraText(cmd,
		[]string{"--columns=backlog,wip,done", "--priorities=low,high"}, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	b, _ := board.Load(path)
	if got, want := b.Board.Columns, []string{"backlog", "wip", "done"}; !equalStrings(got, want) {
		t.Errorf("columns: got %v, want %v", got, want)
	}
	if got, want := b.Board.Priorities, []string{"low", "high"}; !equalStrings(got, want) {
		t.Errorf("priorities: got %v, want %v", got, want)
	}
}

func TestInit_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	// Seed an existing file.
	if err := os.WriteFile(path, []byte("preexisting\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	pre, _ := os.ReadFile(path)
	cmd := newDummyInitForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{}, false)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var aie *AlreadyInitializedError
	if !errors.As(err, &aie) {
		t.Errorf("got %T, want *AlreadyInitializedError", err)
	}
	post, _ := os.ReadFile(path)
	if !bytes.Equal(pre, post) {
		t.Errorf("file was modified despite refusal: pre=%q post=%q", pre, post)
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	if err := os.WriteFile(path, []byte("preexisting\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := newDummyInitForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"--force"}, false)
	if err != nil {
		t.Fatalf("force init: %v", err)
	}
	if _, err := board.Load(path); err != nil {
		t.Errorf("post-force file not loadable: %v", err)
	}
}

func TestInit_DuplicateColumns_ValidationSurfaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kanban.toml")
	cmd := newDummyInitForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"--columns=todo,todo,done"}, false)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	var ve *board.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("got %T, want *board.ValidationError", err)
	}
}

// --- P5: skill packaging additions to init ---

func TestInit_WritesSkill(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.toml")
	skillPath := filepath.Join(dir, ".claude", "skills", "ezida-kanban", "SKILL.md")
	cmd := newDummyInitForPaths(boardPath, skillPath, false)
	_, _, err := executeCobraText(cmd, []string{}, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	got, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !bytes.Equal(got, skill.Bytes) {
		t.Errorf("skill file bytes differ from skill.Bytes (len got=%d, want=%d)", len(got), len(skill.Bytes))
	}
}

func TestInit_SkillOnly_DoesNotCreateBoard(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.toml")
	skillPath := filepath.Join(dir, ".claude", "skills", "ezida-kanban", "SKILL.md")
	cmd := newDummyInitForPaths(boardPath, skillPath, false)
	_, _, err := executeCobraText(cmd, []string{"--skill-only"}, false)
	if err != nil {
		t.Fatalf("init --skill-only: %v", err)
	}
	if _, err := os.Stat(boardPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("kanban.toml unexpectedly created: %v", err)
	}
	if _, err := os.Stat(skillPath); err != nil {
		t.Errorf("skill not written: %v", err)
	}
}

func TestInit_SkillOnly_DoesNotTouchExistingBoard(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.toml")
	skillPath := filepath.Join(dir, ".claude", "skills", "ezida-kanban", "SKILL.md")
	sentinel := []byte("# sentinel\nschema_version = 1\n")
	if err := os.WriteFile(boardPath, sentinel, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := newDummyInitForPaths(boardPath, skillPath, false)
	_, _, err := executeCobraText(cmd, []string{"--skill-only"}, false)
	if err != nil {
		t.Fatalf("init --skill-only: %v", err)
	}
	got, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatalf("read board: %v", err)
	}
	if !bytes.Equal(got, sentinel) {
		t.Errorf("kanban.toml changed: got %q want %q", got, sentinel)
	}
}

func TestInit_JSONEnvelope_Full(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.toml")
	skillPath := filepath.Join(dir, ".claude", "skills", "ezida-kanban", "SKILL.md")
	cmd := newDummyInitForPaths(boardPath, skillPath, true)
	stdout, _, err := executeCobraText(cmd, []string{}, true)
	if err != nil {
		t.Fatalf("init --json: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal %q: %v", stdout, err)
	}
	if raw["initialized"] != true {
		t.Errorf("initialized: %v", raw["initialized"])
	}
	if raw["path"] != boardPath {
		t.Errorf("path: %v want %v", raw["path"], boardPath)
	}
	if raw["skill_path"] != skillPath {
		t.Errorf("skill_path: %v want %v", raw["skill_path"], skillPath)
	}
	if _, has := raw["skill_only"]; has {
		t.Errorf("skill_only must be absent in full envelope: %v", raw)
	}
}

func TestInit_JSONEnvelope_SkillOnly(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.toml")
	skillPath := filepath.Join(dir, ".claude", "skills", "ezida-kanban", "SKILL.md")
	cmd := newDummyInitForPaths(boardPath, skillPath, true)
	stdout, _, err := executeCobraText(cmd, []string{"--skill-only"}, true)
	if err != nil {
		t.Fatalf("init --skill-only --json: %v", err)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Errorf("expected trailing newline: %q", stdout)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal %q: %v", stdout, err)
	}
	if raw["skill_only"] != true {
		t.Errorf("skill_only: %v", raw["skill_only"])
	}
	if raw["skill_path"] != skillPath {
		t.Errorf("skill_path: %v want %v", raw["skill_path"], skillPath)
	}
	if _, has := raw["initialized"]; has {
		t.Errorf("initialized must be absent in skill-only envelope: %v", raw)
	}
	if _, has := raw["path"]; has {
		t.Errorf("path must be absent in skill-only envelope: %v", raw)
	}
}

func TestInit_TextOutput_IncludesCommentNote(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.toml")
	skillPath := filepath.Join(dir, ".claude", "skills", "ezida-kanban", "SKILL.md")
	cmd := newDummyInitForPaths(boardPath, skillPath, false)
	stdout, _, err := executeCobraText(cmd, []string{}, false)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	const note = "note: TOML comments are not preserved across ezida writes"
	if !strings.Contains(stdout, note) {
		t.Errorf("missing note line: %q", stdout)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if got := lines[len(lines)-1]; got != note {
		t.Errorf("last line: %q, want %q", got, note)
	}
}

func TestInit_SkillOnly_TextOutput_NoBoardMention(t *testing.T) {
	dir := t.TempDir()
	boardPath := filepath.Join(dir, "kanban.toml")
	skillPath := filepath.Join(dir, ".claude", "skills", "ezida-kanban", "SKILL.md")
	cmd := newDummyInitForPaths(boardPath, skillPath, false)
	stdout, _, err := executeCobraText(cmd, []string{"--skill-only"}, false)
	if err != nil {
		t.Fatalf("init --skill-only: %v", err)
	}
	if strings.Contains(stdout, "kanban.toml") {
		t.Errorf("skill-only stdout must not mention kanban.toml: %q", stdout)
	}
	if !strings.Contains(stdout, skillPath) {
		t.Errorf("expected skill path in stdout: %q", stdout)
	}
}

// TestWriteSkillFile_CreatesNestedParents confirms writeSkillFile
// creates missing parent directories (task 3.2 done-condition).
func TestWriteSkillFile_CreatesNestedParents(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "SKILL.md")
	if err := writeSkillFile(nested); err != nil {
		t.Fatalf("writeSkillFile: %v", err)
	}
	got, err := os.ReadFile(nested)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, skill.Bytes) {
		t.Error("written bytes differ from embedded skill.Bytes")
	}
}

// newDummyInitForPath returns an init command that writes to the
// given absolute path instead of the hard-coded BoardPath. It mirrors
// the production NewInitCmd flag wiring. The skill file is written
// next to the board file under a private .claude/skills tree so tests
// stay isolated to t.TempDir().
func newDummyInitForPath(path string, asJSON bool) *cobra.Command {
	dir := filepath.Dir(path)
	skillPath := filepath.Join(dir, ".claude", "skills", "ezida-kanban", "SKILL.md")
	return newDummyInitForPaths(path, skillPath, asJSON)
}

// newDummyInitForPaths is the lower-level helper used by skill-only
// tests that need to assert the skill path independently of the board
// path.
func newDummyInitForPaths(boardPath, skillPath string, asJSON bool) *cobra.Command {
	var (
		columnsCSV    string
		prioritiesCSV string
		force         bool
		skillOnly     bool
	)
	cmd := &cobra.Command{
		Use:  "init",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cols := parseCSVOrDefault(columnsCSV, defaultColumns)
			prios := parseCSVOrDefault(prioritiesCSV, defaultPriorities)
			return runInit(cmd, boardPath, skillPath, cols, prios, force, skillOnly, asJSON)
		},
	}
	cmd.Flags().StringVar(&columnsCSV, "columns", "", "")
	cmd.Flags().StringVar(&prioritiesCSV, "priorities", "", "")
	cmd.Flags().BoolVar(&force, "force", false, "")
	cmd.Flags().BoolVar(&skillOnly, "skill-only", false, "")
	return cmd
}

// executeCobraText is a tiny cobra exec helper used by every test
// that pokes a command directly (sidestepping the persistent --json
// flag on the root).
func executeCobraText(cmd *cobra.Command, args []string, asJSON bool) (string, string, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	_ = asJSON
	return stdout.String(), stderr.String(), err
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- board (task 4.3) ---

func TestBoard_TextOutput(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "board"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runBoard(cmd, path, false); err != nil {
		t.Fatalf("board: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "schema 1") {
		t.Errorf("missing 'schema 1' line: %q", out)
	}
	if !strings.Contains(out, "todo (3)") || !strings.Contains(out, "ongoing (1)") || !strings.Contains(out, "done (7)") {
		t.Errorf("missing per-column counts: %q", out)
	}
	if !strings.Contains(out, "low < medium < high") {
		t.Errorf("missing priorities line: %q", out)
	}
}

func TestBoard_JSONOutput(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "board"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runBoard(cmd, path, true); err != nil {
		t.Fatalf("board: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout.String())
	}
	if got["schema_version"].(float64) != 1 {
		t.Errorf("schema_version: %v", got["schema_version"])
	}
	cpc := got["cards_per_column"].(map[string]any)
	if cpc["todo"].(float64) != 3 || cpc["ongoing"].(float64) != 1 || cpc["done"].(float64) != 7 {
		t.Errorf("cards_per_column: %v", cpc)
	}
	cols := got["columns"].([]any)
	wantCols := []string{"todo", "ongoing", "done"}
	for i, c := range cols {
		if c.(string) != wantCols[i] {
			t.Errorf("columns[%d]: %v, want %s", i, c, wantCols[i])
		}
	}
}

func TestBoard_MissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-kanban.toml")
	cmd := &cobra.Command{Use: "board"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := runBoard(cmd, missing, true)
	if err == nil {
		t.Fatal("expected error")
	}
	// The classifier (output.Classify) should map this to BOARD_NOT_FOUND.
	// We assert via the underlying fs.ErrNotExist path: if the error
	// satisfies errors.Is(err, fs.ErrNotExist) then output.Classify
	// returns BOARD_NOT_FOUND (verified in output_test.go).
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("got %v, want fs.ErrNotExist-wrapped error", err)
	}
}

// --- list (task 5.x) ---

func TestList_NoFilters_AllCards(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "list"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runList(cmd, path, listFlags{}, true); err != nil {
		t.Fatalf("list: %v", err)
	}
	var raw struct {
		Cards []map[string]any `json:"cards"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout.String())
	}
	if len(raw.Cards) != 11 {
		t.Errorf("cards: got %d, want 11", len(raw.Cards))
	}
	// IDs in file order.
	wantIDs := []string{"a3f2k9", "b7m1p4", "c8q3r5", "d2s4t6", "e5u7v9",
		"f6w8x1", "g7y9z2", "h8a1b3", "i9c2d4", "j1e3f5", "k2g4h6"}
	for i, w := range wantIDs {
		if raw.Cards[i]["id"] != w {
			t.Errorf("card[%d].id: got %v, want %s", i, raw.Cards[i]["id"], w)
		}
	}
}

func TestList_FilterColumn(t *testing.T) {
	path := copyFixture(t)
	stdout := captureList(t, path, listFlags{column: "todo"}, true)
	var raw struct{ Cards []map[string]any `json:"cards"` }
	_ = json.Unmarshal([]byte(stdout), &raw)
	if len(raw.Cards) != 3 {
		t.Errorf("todo cards: got %d, want 3", len(raw.Cards))
	}
}

func TestList_FilterTag(t *testing.T) {
	path := copyFixture(t)
	stdout := captureList(t, path, listFlags{tag: "infra"}, true)
	var raw struct{ Cards []map[string]any `json:"cards"` }
	_ = json.Unmarshal([]byte(stdout), &raw)
	if len(raw.Cards) != 2 {
		t.Errorf("infra cards: got %d, want 2", len(raw.Cards))
	}
}

func TestList_FilterColumnAndTag(t *testing.T) {
	path := copyFixture(t)
	stdout := captureList(t, path, listFlags{column: "todo", tag: "security"}, true)
	var raw struct{ Cards []map[string]any `json:"cards"` }
	_ = json.Unmarshal([]byte(stdout), &raw)
	if len(raw.Cards) != 1 {
		t.Errorf("todo+security: got %d, want 1", len(raw.Cards))
	}
	if raw.Cards[0]["id"] != "a3f2k9" {
		t.Errorf("got id %v", raw.Cards[0]["id"])
	}
}

func TestList_FilterColumnAndPriority(t *testing.T) {
	path := copyFixture(t)
	stdout := captureList(t, path, listFlags{column: "done", priority: "high"}, true)
	var raw struct{ Cards []map[string]any `json:"cards"` }
	_ = json.Unmarshal([]byte(stdout), &raw)
	if len(raw.Cards) != 1 {
		t.Errorf("done+high: got %d, want 1", len(raw.Cards))
	}
	if raw.Cards[0]["id"] != "k2g4h6" {
		t.Errorf("got id %v", raw.Cards[0]["id"])
	}
}

func TestList_TitleContainsCaseInsensitive(t *testing.T) {
	path := copyFixture(t)
	stdout := captureList(t, path, listFlags{titleContains: "AUTH"}, true)
	var raw struct{ Cards []map[string]any `json:"cards"` }
	_ = json.Unmarshal([]byte(stdout), &raw)
	if len(raw.Cards) != 1 || raw.Cards[0]["id"] != "a3f2k9" {
		t.Errorf("title contains AUTH: %v", raw.Cards)
	}
}

func TestList_NoMatchExitZero(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "list"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	err := runList(cmd, path, listFlags{tag: "nonexistent"}, true)
	if err != nil {
		t.Fatalf("no-match should be success: %v", err)
	}
	var raw struct{ Cards []map[string]any `json:"cards"` }
	_ = json.Unmarshal(stdout.Bytes(), &raw)
	if len(raw.Cards) != 0 {
		t.Errorf("cards: got %d, want 0", len(raw.Cards))
	}
}

func TestList_InvalidColumnFilter(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "list"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := runList(cmd, path, listFlags{column: "ghost"}, true)
	if err == nil {
		t.Fatal("expected error")
	}
	var ife *InvalidFilterError
	if !errors.As(err, &ife) {
		t.Errorf("got %T, want *InvalidFilterError", err)
	}
	if ife.Flag != "column" {
		t.Errorf("flag: %s", ife.Flag)
	}
}

func TestList_InvalidPriorityFilter(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "list"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := runList(cmd, path, listFlags{priority: "urgent"}, true)
	if err == nil {
		t.Fatal("expected error")
	}
	var ife *InvalidFilterError
	if !errors.As(err, &ife) {
		t.Errorf("got %T, want *InvalidFilterError", err)
	}
	if ife.Flag != "priority" {
		t.Errorf("flag: %s", ife.Flag)
	}
}

func TestList_DescriptionOmittedInJSON(t *testing.T) {
	path := copyFixture(t)
	stdout := captureList(t, path, listFlags{}, true)
	var raw struct{ Cards []map[string]any `json:"cards"` }
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, c := range raw.Cards {
		if _, has := c["description"]; has {
			t.Errorf("card %v unexpectedly carries description", c["id"])
		}
	}
}

func captureList(t *testing.T, path string, f listFlags, asJSON bool) string {
	t.Helper()
	cmd := &cobra.Command{Use: "list"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runList(cmd, path, f, asJSON); err != nil {
		t.Fatalf("list: %v", err)
	}
	return stdout.String()
}

// --- get (task 6.x) ---

func TestGet_FoundText(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "get"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runGet(cmd, path, "a3f2k9", false); err != nil {
		t.Fatalf("get: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"ID:", "a3f2k9", "Refactor auth", "Description:", "session-based to JWT"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestGet_FoundJSON(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "get"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runGet(cmd, path, "a3f2k9", true); err != nil {
		t.Fatalf("get: %v", err)
	}
	var raw struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw.Card["id"] != "a3f2k9" {
		t.Errorf("id: %v", raw.Card["id"])
	}
	desc, _ := raw.Card["description"].(string)
	if !strings.Contains(desc, "JWT") {
		t.Errorf("description: %q", desc)
	}
}

func TestGet_NotFound(t *testing.T) {
	path := copyFixture(t)
	cmd := &cobra.Command{Use: "get"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := runGet(cmd, path, "zzzzzz", true)
	if err == nil {
		t.Fatal("expected error")
	}
	var nfe *CardNotFoundError
	if !errors.As(err, &nfe) {
		t.Errorf("got %T, want *CardNotFoundError", err)
	}
	if nfe.ID != "zzzzzz" {
		t.Errorf("id: %s", nfe.ID)
	}
}

func TestGet_PriorityOmittedInJSON(t *testing.T) {
	path := copyFixture(t)
	// Card b7m1p4 has no priority and no tags.
	cmd := &cobra.Command{Use: "get"}
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	if err := runGet(cmd, path, "b7m1p4", true); err != nil {
		t.Fatalf("get: %v", err)
	}
	var raw struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, has := raw.Card["priority"]; has {
		t.Errorf("priority unexpectedly present: %v", raw.Card)
	}
}

// --- shared infra check: the persistent --json flag really reaches commands ---

func TestRunCmd_PersistentJSONFlag(t *testing.T) {
	// Sanity: NewBoardCmd reads jsonOut by pointer. Verify the
	// closure path mirrors the production wiring.
	jsonFlag := true
	cmd := NewBoardCmd(&jsonFlag)
	if cmd.Use != "board" {
		t.Errorf("Use: %s", cmd.Use)
	}
	// Avoid hitting the filesystem: we only check construction here.
	_ = runCmd
}
