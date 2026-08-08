package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

func TestColumnsDone_MarksAColumnTerminal(t *testing.T) {
	path := copyFixture(t) // columns: todo, ongoing, done — none terminal
	if _, _, err := runColumnsCmd(t, path, false, "done", "ongoing"); err != nil {
		t.Fatalf("columns done: %v", err)
	}
	b := loadBoard(t, path)
	if !b.Board.IsDoneColumn("ongoing") {
		t.Error("ongoing was not marked terminal")
	}
	if !strings.Contains(readFile(t, path), "'ongoing*'") {
		t.Errorf("the marker was not written:\n%s", readFile(t, path))
	}
	// Cards keep the bare name.
	for _, c := range b.Cards {
		if strings.Contains(c.Column, "*") {
			t.Errorf("card %s stored a marked column name %q", c.ID, c.Column)
		}
	}
}

func TestColumnsUndone_ClearsTheMarker(t *testing.T) {
	path := copyEpicsFixture(t) // done* is terminal
	if _, _, err := runColumnsCmd(t, path, false, "undone", "done"); err != nil {
		t.Fatalf("columns undone: %v", err)
	}
	if loadBoard(t, path).Board.IsDoneColumn("done") {
		t.Error("done is still terminal")
	}
	if strings.Contains(readFile(t, path), "'done*'") {
		t.Error("the marker survived")
	}
}

// Both directions are idempotent, down to the bytes on disk.
func TestColumnsDone_IsIdempotent(t *testing.T) {
	cases := []struct {
		name    string
		fixture func(t *testing.T) string
		sub     string
		column  string
	}{
		{"already terminal", copyEpicsFixture, "done", "done"},
		{"already plain", copyFixture, "undone", "todo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.fixture(t)
			before := readFile(t, path)
			if _, _, err := runColumnsCmd(t, path, false, tc.sub, tc.column); err != nil {
				t.Fatalf("columns %s: %v", tc.sub, err)
			}
			if readFile(t, path) != before {
				t.Error("kanban.toml changed on a no-op")
			}
		})
	}
}

func TestColumnsDone_RejectsUnknownAndMarkedNames(t *testing.T) {
	cases := []struct{ name, arg string }{
		{"unknown column", "ghost"},
		// The suffix is a file-format detail; passing it means naming a
		// column that does not exist.
		{"suffix as argument", "done*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := copyEpicsFixture(t)
			before := readFile(t, path)
			_, _, err := runColumnsCmd(t, path, false, "done", tc.arg)
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if got := AsDetailed(err).Code(); got != "COLUMN_NOT_FOUND" {
				t.Errorf("code = %s, want COLUMN_NOT_FOUND", got)
			}
			if readFile(t, path) != before {
				t.Error("kanban.toml was modified by a refused command")
			}
		})
	}
}

func TestColumnsRename_CarriesTheTerminalMarker(t *testing.T) {
	path := copyEpicsFixture(t)
	if _, _, err := runColumnsCmd(t, path, false, "rename", "done", "shipped"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	b := loadBoard(t, path)
	if !b.Board.IsDoneColumn("shipped") {
		t.Error("the renamed column lost its terminal status")
	}
	if !strings.Contains(readFile(t, path), "'shipped*'") {
		t.Errorf("marker not written:\n%s", readFile(t, path))
	}
	// The rename still propagates to cards, with a bare name.
	for _, c := range b.Cards {
		if c.Column == "done" {
			t.Errorf("card %s still references the old name", c.ID)
		}
	}
}

func TestColumnsRename_RejectsAMarkedTarget(t *testing.T) {
	path := copyEpicsFixture(t)
	before := readFile(t, path)
	_, _, err := runColumnsCmd(t, path, false, "rename", "todo", "later*")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if got := AsDetailed(err).Code(); got != "INVALID_COLUMN_NAME" {
		t.Errorf("code = %s, want INVALID_COLUMN_NAME", got)
	}
	if readFile(t, path) != before {
		t.Error("kanban.toml was modified by a refused command")
	}
}

func TestColumnsAdd_RejectsAMarkedName(t *testing.T) {
	path := copyEpicsFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "add", "later*")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if got := AsDetailed(err).Code(); got != "INVALID_COLUMN_NAME" {
		t.Errorf("code = %s, want INVALID_COLUMN_NAME", got)
	}
}

func TestColumnsList_MarksTerminalColumns(t *testing.T) {
	path := copyEpicsFixture(t)
	stdout, _, err := runColumnsCmd(t, path, false)
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	var doneLine, todoLine string
	for _, line := range strings.Split(stdout, "\n") {
		switch {
		case strings.HasPrefix(line, "done"):
			doneLine = line
		case strings.HasPrefix(line, "todo"):
			todoLine = line
		}
	}
	if doneLine == "" || todoLine == "" {
		t.Fatalf("listing did not cover both columns:\n%s", stdout)
	}
	if !strings.Contains(doneLine, doneMarker) {
		t.Errorf("terminal column carries no indicator: %q", doneLine)
	}
	if strings.Contains(todoLine, doneMarker) {
		t.Errorf("plain column carries an indicator: %q", todoLine)
	}
	// Counts are part of the listing.
	if !strings.Contains(stdout, "CARDS") {
		t.Errorf("listing has no count column:\n%s", stdout)
	}
	if strings.Contains(stdout, "*") {
		t.Errorf("the file-format marker leaked into the listing:\n%s", stdout)
	}
}

func TestColumnsList_JSONShape(t *testing.T) {
	path := copyEpicsFixture(t)
	stdout, _, err := runColumnsCmd(t, path, true)
	if err != nil {
		t.Fatalf("columns: %v", err)
	}
	var env struct {
		Columns []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
			Done  bool   `json:"done"`
		} `json:"columns"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, stdout)
	}
	if len(env.Columns) != 3 {
		t.Fatalf("got %d columns, want 3", len(env.Columns))
	}
	for _, c := range env.Columns {
		switch c.Name {
		case "done":
			if !c.Done || c.Count != 1 {
				t.Errorf("done entry = %+v, want done with 1 card", c)
			}
		case "backlog":
			if c.Done || c.Count != 2 {
				t.Errorf("backlog entry = %+v, want plain with 2 cards", c)
			}
		}
	}
}

// ---------------------------------------------------------------- init

func TestInit_WritesATerminalColumnAndTheComment(t *testing.T) {
	cases := []struct {
		name    string
		columns string
		want    string
	}{
		{"defaults prefer a column named done", "", "'todo', 'ongoing', 'done*'"},
		{"custom columns prefer done", "backlog,wip,done", "'backlog', 'wip', 'done*'"},
		{"no done falls back to the last column", "todo,wip,shipped", "'todo', 'wip', 'shipped*'"},
		{"explicit markers suppress the automatic choice", "todo,shipped*,wont-fix*",
			"'todo', 'shipped*', 'wont-fix*'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := dir + "/kanban.toml"
			cmd := newDummyInitForPath(path, false)
			args := []string{}
			if tc.columns != "" {
				args = append(args, "--columns="+tc.columns)
			}
			if _, _, err := executeCobraText(cmd, args, false); err != nil {
				t.Fatalf("init: %v", err)
			}
			body := readFile(t, path)
			if !strings.Contains(body, tc.want) {
				t.Errorf("columns line missing %q:\n%s", tc.want, body)
			}
			if !strings.Contains(body, board.ColumnsComment) {
				t.Errorf("the explanatory comment was not written:\n%s", body)
			}
			b, err := board.Load(path)
			if err != nil {
				t.Fatalf("the fresh board does not load: %v", err)
			}
			if b.SchemaVersion != board.SupportedSchemaVersion {
				t.Errorf("schema_version = %d, want %d", b.SchemaVersion, board.SupportedSchemaVersion)
			}
			if len(b.Board.DoneColumns()) == 0 {
				t.Error("no column was marked terminal")
			}
		})
	}
}
