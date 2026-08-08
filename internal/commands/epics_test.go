package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// copyNamedFixture copies the named testdata file into a fresh temp
// directory as kanban.toml and returns the path.
func copyNamedFixture(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "kanban.toml")
	if err := os.WriteFile(dst, src, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}
	return dst
}

// copyEpicsFixture is the shorthand used by every test below.
func copyEpicsFixture(t *testing.T) string { return copyNamedFixture(t, "epics.toml") }

// loadBoard re-reads a board after a command has written it.
func loadBoard(t *testing.T, path string) *board.Board {
	t.Helper()
	b, err := board.Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	return b
}

// cardByID returns the card with the given id, failing the test when
// it is absent.
func cardByID(t *testing.T, b *board.Board, id string) board.Card {
	t.Helper()
	idx := indexCardByID(b.Cards, id)
	if idx < 0 {
		t.Fatalf("card %s not found", id)
	}
	return b.Cards[idx]
}

// readFile returns a file's bytes as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(buf)
}

// ---------------------------------------------------------------- add

func TestAdd_UnderAnEpicColorsTheParent(t *testing.T) {
	path := copyEpicsFixture(t)
	// Strip the parent's color so the command has to assign one.
	b := loadBoard(t, path)
	b.Cards[0].Color = ""
	if err := board.Save(path, b); err != nil {
		t.Fatalf("seed: %v", err)
	}
	before := cardByID(t, loadBoard(t, path), "rl4m9x").UpdatedAt

	cmd := newDummyAddForPath(path, false)
	stdout, _, err := executeCobraText(cmd,
		[]string{"Card labels", "--column=backlog", "--epic=rl4m9x"}, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	after := loadBoard(t, path)
	newID := strings.TrimSpace(stdout)
	child := cardByID(t, after, newID)
	if child.Epic != "rl4m9x" {
		t.Errorf("child epic = %q, want rl4m9x", child.Epic)
	}
	parent := cardByID(t, after, "rl4m9x")
	if parent.Color == "" {
		t.Error("parent did not acquire a color")
	}
	if !parent.UpdatedAt.After(before) && parent.UpdatedAt.Equal(before) {
		t.Error("parent's updated_at was not refreshed by the color assignment")
	}
}

func TestAdd_ExistingParentColorSurvives(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := newDummyAddForPath(path, false)
	if _, _, err := executeCobraText(cmd,
		[]string{"Card labels", "--column=backlog", "--epic=rl4m9x"}, false); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := cardByID(t, loadBoard(t, path), "rl4m9x").Color; got != "#8b5cf6" {
		t.Fatalf("parent color = %q, want #8b5cf6", got)
	}
}

func TestAdd_EpicRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		code string
	}{
		{
			name: "unknown epic",
			args: []string{"Something", "--column=todo", "--epic=zzzzzz"},
			code: "INVALID_EPIC",
		},
		{
			// f20wbo is itself a child, so it cannot become a parent.
			name: "epic that is itself a child",
			args: []string{"Something", "--column=todo", "--epic=f20wbo"},
			code: "INVALID_EPIC",
		},
		{
			name: "malformed color",
			args: []string{"Something", "--column=todo", "--color=chartreuse"},
			code: "INVALID_COLOR",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := copyEpicsFixture(t)
			before := readFile(t, path)
			cmd := newDummyAddForPath(path, false)
			_, _, err := executeCobraText(cmd, tc.args, false)
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if got := AsDetailed(err).Code(); got != tc.code {
				t.Errorf("code = %s, want %s", got, tc.code)
			}
			if readFile(t, path) != before {
				t.Error("kanban.toml was modified by a refused command")
			}
		})
	}
}

func TestAdd_ColorByPaletteName(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := newDummyAddForPath(path, false)
	stdout, _, err := executeCobraText(cmd,
		[]string{"Standalone", "--column=todo", "--color=emerald"}, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	got := cardByID(t, loadBoard(t, path), strings.TrimSpace(stdout)).Color
	if got != "#10b981" {
		t.Fatalf("color = %q, want #10b981", got)
	}
	// The palette name itself never reaches the file.
	if strings.Contains(readFile(t, path), "emerald") {
		t.Error("the palette name was written to disk")
	}
}

// --------------------------------------------------------------- edit

func TestEdit_AssignEpicColorsTheParent(t *testing.T) {
	path := copyEpicsFixture(t)
	b := loadBoard(t, path)
	b.Cards[0].Color = ""
	if err := board.Save(path, b); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd := newDummyEditForPath(path, false)
	if _, _, err := executeCobraText(cmd, []string{"a3f2k9", "--epic=rl4m9x"}, false); err != nil {
		t.Fatalf("edit: %v", err)
	}
	after := loadBoard(t, path)
	if got := cardByID(t, after, "a3f2k9").Epic; got != "rl4m9x" {
		t.Errorf("epic = %q, want rl4m9x", got)
	}
	if cardByID(t, after, "rl4m9x").Color == "" {
		t.Error("parent did not acquire a color")
	}
}

func TestEdit_ClearEpicKeepsTheParentColor(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := newDummyEditForPath(path, false)
	if _, _, err := executeCobraText(cmd, []string{"f20wbo", "--no-epic"}, false); err != nil {
		t.Fatalf("edit: %v", err)
	}
	after := loadBoard(t, path)
	if got := cardByID(t, after, "f20wbo").Epic; got != "" {
		t.Errorf("epic = %q, want empty", got)
	}
	if !strings.Contains(readFile(t, path), "color = '#8b5cf6'") {
		t.Error("the parent lost its color")
	}
}

func TestEdit_NoColorClearsTheField(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := newDummyEditForPath(path, false)
	if _, _, err := executeCobraText(cmd, []string{"rl4m9x", "--no-color"}, false); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got := cardByID(t, loadBoard(t, path), "rl4m9x").Color; got != "" {
		t.Fatalf("color = %q, want empty", got)
	}
}

func TestEdit_EpicRefusals(t *testing.T) {
	cases := []struct {
		name string
		args []string
		code string
	}{
		{"unknown epic", []string{"a3f2k9", "--epic=zzzzzz"}, "INVALID_EPIC"},
		{"self reference", []string{"a3f2k9", "--epic=a3f2k9"}, "INVALID_EPIC"},
		{"nested epic", []string{"a3f2k9", "--epic=f20wbo"}, "INVALID_EPIC"},
		{"contradictory epic flags", []string{"f20wbo", "--epic=rl4m9x", "--no-epic"}, "INVALID_EPIC"},
		{"malformed color", []string{"rl4m9x", "--color=chartreuse"}, "INVALID_COLOR"},
		{"contradictory color flags", []string{"rl4m9x", "--color=violet", "--no-color"}, "INVALID_COLOR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := copyEpicsFixture(t)
			before := readFile(t, path)
			cmd := newDummyEditForPath(path, false)
			_, _, err := executeCobraText(cmd, tc.args, false)
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if got := AsDetailed(err).Code(); got != tc.code {
				t.Errorf("code = %s, want %s", got, tc.code)
			}
			if readFile(t, path) != before {
				t.Error("kanban.toml was modified by a refused command")
			}
		})
	}
}

// The nesting refusal must explain itself: the target looks like an
// ordinary card to the user.
func TestEdit_NestedEpicRefusalExplainsItself(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := newDummyEditForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9", "--epic=f20wbo"}, false)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	msg := err.Error()
	for _, want := range []string{"already belongs", "one level"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not mention %q", msg, want)
		}
	}
	details, _ := AsDetailed(err).Details().(map[string]any)
	if details["epic"] != "f20wbo" {
		t.Errorf("details = %v, want the rejected id", details)
	}
}

func TestEdit_NothingToEditMentionsTheNewFlags(t *testing.T) {
	msg := (&NothingToEditError{}).Error()
	for _, flag := range []string{"--epic", "--no-epic", "--color", "--no-color"} {
		if !strings.Contains(msg, flag) {
			t.Errorf("NOTHING_TO_EDIT message does not mention %s", flag)
		}
	}
}

// ----------------------------------------------------------------- rm

func TestRm_DeletingAParentOrphansItsChildren(t *testing.T) {
	path := copyEpicsFixture(t)
	stamps := map[string]string{}
	for _, c := range loadBoard(t, path).Cards {
		stamps[c.ID] = c.UpdatedAt.String()
	}

	stderr := &bytes.Buffer{}
	rio := rmIO{in: strings.NewReader(""), err: stderr, interactive: false}
	cmd, _ := newDummyRmForPath(path, false, rio)
	stdout, _, err := executeCobraText(cmd, []string{"rl4m9x", "--yes"}, false)
	if err != nil {
		t.Fatalf("rm: %v", err)
	}
	if !strings.Contains(stdout, "removed rl4m9x") {
		t.Errorf("stdout = %q", stdout)
	}

	after := loadBoard(t, path)
	if indexCardByID(after.Cards, "rl4m9x") >= 0 {
		t.Error("the parent survived the delete")
	}
	for _, c := range after.Cards {
		if c.Epic != "" {
			t.Errorf("card %s still references the deleted epic", c.ID)
		}
		if stamps[c.ID] != c.UpdatedAt.String() {
			t.Errorf("card %s had its updated_at refreshed by orphaning", c.ID)
		}
	}
	// The write is reported, never silent.
	report := stderr.String()
	for _, id := range []string{"f20wbo", "wrshlo", "q7t6z2"} {
		if !strings.Contains(report, id) {
			t.Errorf("stderr does not name orphaned card %s: %q", id, report)
		}
	}
}

func TestRm_OrphanedIDsInJSONFollowFileOrder(t *testing.T) {
	path := copyEpicsFixture(t)
	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: false}
	cmd, _ := newDummyRmForPath(path, true, rio)
	stdout, _, err := executeCobraText(cmd, []string{"rl4m9x", "--yes"}, true)
	if err != nil {
		t.Fatalf("rm: %v", err)
	}
	var got struct {
		ID       string   `json:"id"`
		Deleted  bool     `json:"deleted"`
		Orphaned []string `json:"orphaned"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, stdout)
	}
	want := []string{"f20wbo", "wrshlo", "q7t6z2"}
	if !equalStrings(got.Orphaned, want) {
		t.Fatalf("orphaned = %v, want %v (board file order)", got.Orphaned, want)
	}
}

func TestRm_DeletingAChildLeavesTheParentUntouched(t *testing.T) {
	path := copyEpicsFixture(t)
	before := cardByID(t, loadBoard(t, path), "rl4m9x")

	rio := rmIO{in: strings.NewReader(""), err: &bytes.Buffer{}, interactive: false}
	cmd, _ := newDummyRmForPath(path, false, rio)
	if _, _, err := executeCobraText(cmd, []string{"f20wbo", "--yes"}, false); err != nil {
		t.Fatalf("rm: %v", err)
	}

	after := cardByID(t, loadBoard(t, path), "rl4m9x")
	if after.Color != before.Color {
		t.Errorf("parent color changed: %q → %q", before.Color, after.Color)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("parent updated_at changed: %v → %v", before.UpdatedAt, after.UpdatedAt)
	}
}

// --------------------------------------------------------------- list

func TestList_EpicFilterIncludesTheParent(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := &cobra.Command{
		Use: "list", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, path, listFlags{epic: "rl4m9x"}, true)
		},
	}
	stdout, _, err := executeCobraText(cmd, nil, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var env struct {
		Cards []struct {
			ID   string `json:"id"`
			Epic string `json:"epic"`
		} `json:"cards"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if len(env.Cards) != 4 {
		t.Fatalf("got %d cards, want 4 (parent + three children)", len(env.Cards))
	}
	var sawParent bool
	for _, c := range env.Cards {
		if c.ID == "rl4m9x" {
			sawParent = true
		}
	}
	if !sawParent {
		t.Error("the epic filter hid the epic itself")
	}
}

func TestList_EpicFilterCombinesWithColumn(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := &cobra.Command{
		Use: "list", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, path, listFlags{epic: "rl4m9x", column: "backlog"}, true)
		},
	}
	stdout, _, err := executeCobraText(cmd, nil, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var env struct {
		Cards []struct {
			ID     string `json:"id"`
			Column string `json:"column"`
		} `json:"cards"`
	}
	_ = json.Unmarshal([]byte(stdout), &env)
	if len(env.Cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(env.Cards))
	}
	for _, c := range env.Cards {
		if c.Column != "backlog" {
			t.Errorf("card %s is in %s, want backlog", c.ID, c.Column)
		}
	}
}

func TestList_UnknownEpicFilterIsAUserError(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := &cobra.Command{
		Use: "list", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, path, listFlags{epic: "zzzzzz"}, true)
		},
	}
	_, _, err := executeCobraText(cmd, nil, true)
	if err == nil {
		t.Fatal("expected INVALID_FILTER")
	}
	if got := AsDetailed(err).Code(); got != "INVALID_FILTER" {
		t.Fatalf("code = %s, want INVALID_FILTER", got)
	}
}

func TestList_JSONCarriesEpicAndColor(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := &cobra.Command{
		Use: "list", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, path, listFlags{}, true)
		},
	}
	stdout, _, err := executeCobraText(cmd, nil, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var env struct {
		Cards []map[string]any `json:"cards"`
	}
	_ = json.Unmarshal([]byte(stdout), &env)
	for _, c := range env.Cards {
		switch c["id"] {
		case "rl4m9x":
			if c["color"] != "#8b5cf6" {
				t.Errorf("parent color = %v", c["color"])
			}
			if _, present := c["epic"]; present {
				t.Error("the parent carries an epic key")
			}
		case "a3f2k9":
			// An unrelated card exposes neither key.
			if _, present := c["epic"]; present {
				t.Error("unrelated card carries an epic key")
			}
			if _, present := c["color"]; present {
				t.Error("unrelated card carries a color key")
			}
		case "f20wbo":
			if c["epic"] != "rl4m9x" {
				t.Errorf("child epic = %v", c["epic"])
			}
		}
		if _, present := c["description"]; present {
			t.Error("list JSON leaked the description")
		}
	}
}

// ---------------------------------------------------------------- get

func TestGet_ChildReportsItsParent(t *testing.T) {
	path := copyEpicsFixture(t)
	stdout := runGetJSON(t, path, "f20wbo")
	epic, ok := stdout["epic"].(map[string]any)
	if !ok {
		t.Fatalf("card.epic = %v, want an object", stdout["epic"])
	}
	if epic["id"] != "rl4m9x" || epic["title"] != "Card relations" {
		t.Errorf("epic = %v", epic)
	}
	if _, present := stdout["children"]; present {
		t.Error("a child reported children")
	}
	if _, present := stdout["progress"]; present {
		t.Error("a child reported progress")
	}
}

func TestGet_ParentReportsChildrenAndProgress(t *testing.T) {
	path := copyEpicsFixture(t)
	card := runGetJSON(t, path, "rl4m9x")
	children, ok := card["children"].([]any)
	if !ok || len(children) != 3 {
		t.Fatalf("children = %v, want three entries", card["children"])
	}
	want := []string{"f20wbo", "wrshlo", "q7t6z2"}
	for i, raw := range children {
		child := raw.(map[string]any)
		if child["id"] != want[i] {
			t.Fatalf("children out of file order: %v", children)
		}
		if child["column"] == nil {
			t.Errorf("child %v carries no column", child["id"])
		}
	}
	progress, ok := card["progress"].(map[string]any)
	if !ok {
		t.Fatalf("progress = %v", card["progress"])
	}
	// wrshlo sits in the terminal column `done`.
	if progress["done"].(float64) != 1 || progress["total"].(float64) != 3 {
		t.Errorf("progress = %v, want 1/3", progress)
	}
	if _, present := card["epic"]; present {
		t.Error("a parent reported an epic")
	}
}

func TestGet_UnrelatedCardReportsNeither(t *testing.T) {
	card := runGetJSON(t, copyEpicsFixture(t), "a3f2k9")
	for _, key := range []string{"epic", "children", "progress"} {
		if _, present := card[key]; present {
			t.Errorf("unrelated card carries %q", key)
		}
	}
}

func TestGet_TextModeReportsTheRelation(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := &cobra.Command{
		Use: "get", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd, path, args[0], false)
		},
	}
	stdout, _, err := executeCobraText(cmd, []string{"rl4m9x"}, false)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, want := range []string{"Card dependencies", "Card due dates", "1/3"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text output missing %q:\n%s", want, stdout)
		}
	}

	childOut, _, err := executeCobraText(&cobra.Command{
		Use: "get", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd, path, args[0], false)
		},
	}, []string{"f20wbo"}, false)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(childOut, "rl4m9x") {
		t.Errorf("child text output does not name its parent:\n%s", childOut)
	}
}

// runGetJSON runs `get <id> --json` against path and returns the card
// object.
func runGetJSON(t *testing.T, path, id string) map[string]any {
	t.Helper()
	cmd := &cobra.Command{
		Use: "get", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd, path, args[0], true)
		},
	}
	stdout, _, err := executeCobraText(cmd, []string{id}, true)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	var env struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, stdout)
	}
	return env.Card
}

// --------------------------------------------------------------- board

func TestBoard_ReportsDoneColumns(t *testing.T) {
	path := copyEpicsFixture(t)
	cmd := &cobra.Command{
		Use: "board", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoard(cmd, path, true)
		},
	}
	stdout, _, err := executeCobraText(cmd, nil, true)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	var env struct {
		Columns     []string `json:"columns"`
		DoneColumns []string `json:"done_columns"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if !equalStrings(env.Columns, []string{"backlog", "todo", "done"}) {
		t.Errorf("columns = %v", env.Columns)
	}
	if !equalStrings(env.DoneColumns, []string{"done"}) {
		t.Errorf("done_columns = %v, want [done]", env.DoneColumns)
	}
	// The suffix never leaks into either mode.
	if strings.Contains(stdout, "*") {
		t.Errorf("the terminal marker leaked into JSON output: %s", stdout)
	}
	text, _, err := executeCobraText(&cobra.Command{
		Use: "board", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoard(cmd, path, false)
		},
	}, nil, false)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if strings.Contains(text, "*") {
		t.Errorf("the terminal marker leaked into text output: %s", text)
	}
	if !strings.Contains(text, doneMarker) {
		t.Errorf("text output carries no terminal indicator:\n%s", text)
	}
}

func TestBoard_NoTerminalColumnsYieldsAnEmptyArray(t *testing.T) {
	path := copyFixture(t) // the populated fixture marks no column
	cmd := &cobra.Command{
		Use: "board", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBoard(cmd, path, true)
		},
	}
	stdout, _, err := executeCobraText(cmd, nil, true)
	if err != nil {
		t.Fatalf("board: %v", err)
	}
	if !strings.Contains(stdout, `"done_columns":[]`) {
		t.Fatalf("done_columns is not an empty array: %s", stdout)
	}
}
