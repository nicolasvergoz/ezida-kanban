package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// newDummyAddForPath builds an add command that writes to the given
// absolute board path. Mirrors NewAddCmd's flag wiring.
func newDummyAddForPath(path string, asJSON bool) *cobra.Command {
	f := addFlags{}
	cmd := &cobra.Command{
		Use:  "add",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, path, args[0], f, asJSON)
		},
	}
	cmd.Flags().StringVar(&f.column, "column", "", "")
	cmd.Flags().StringVar(&f.priority, "priority", "", "")
	cmd.Flags().StringVar(&f.tagsCSV, "tags", "", "")
	cmd.Flags().StringVar(&f.description, "description", "", "")
	return cmd
}

func TestAdd_HelpListsAllFlags(t *testing.T) {
	jsonFlag := false
	cmd := NewAddCmd(&jsonFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	_ = cmd.Execute()
	out := buf.String()
	for _, flag := range []string{"--column", "--priority", "--tags", "--description"} {
		if !strings.Contains(out, flag) {
			t.Errorf("help missing %s flag:\n%s", flag, out)
		}
	}
}

func TestAdd_HappyPath(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyAddForPath(path, false)
	stdout, _, err := executeCobraText(cmd, []string{"New task", "--column=todo"}, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	id := strings.TrimSpace(stdout)
	if len(id) != 6 {
		t.Errorf("id length: got %q", id)
	}
	b, _ := board.Load(path)
	var found *board.Card
	for i := range b.Cards {
		if b.Cards[i].ID == id {
			found = &b.Cards[i]
		}
	}
	if found == nil {
		t.Fatalf("new card not found in board")
	}
	if found.Title != "New task" || found.Column != "todo" {
		t.Errorf("card: %+v", found)
	}
	if !found.CreatedAt.Equal(found.UpdatedAt) {
		t.Errorf("created != updated: %v vs %v", found.CreatedAt, found.UpdatedAt)
	}
}

func TestAdd_UnknownColumn(t *testing.T) {
	path := copyFixture(t)
	pre, _ := os.ReadFile(path)
	cmd := newDummyAddForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"Title", "--column=ghost"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var cnf *ColumnNotFoundError
	if !errors.As(err, &cnf) {
		t.Errorf("got %T, want *ColumnNotFoundError", err)
	}
	post, _ := os.ReadFile(path)
	if !bytes.Equal(pre, post) {
		t.Errorf("file modified despite error")
	}
}

func TestAdd_UnknownPriority(t *testing.T) {
	path := copyFixture(t)
	pre, _ := os.ReadFile(path)
	cmd := newDummyAddForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"Title", "--column=todo", "--priority=urgent"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var ipe *InvalidPriorityError
	if !errors.As(err, &ipe) {
		t.Errorf("got %T, want *InvalidPriorityError", err)
	}
	post, _ := os.ReadFile(path)
	if !bytes.Equal(pre, post) {
		t.Errorf("file modified despite error")
	}
}

func TestAdd_EmptyTitle(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyAddForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"", "--column=todo"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var mte *MissingTitleError
	if !errors.As(err, &mte) {
		t.Errorf("got %T, want *MissingTitleError", err)
	}
}

func TestAdd_TagError(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyAddForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"Title", "--column=todo", "--tags=,security,"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var ite *InvalidTagError
	if !errors.As(err, &ite) {
		t.Errorf("got %T, want *InvalidTagError", err)
	}
}

func TestAdd_AppendsToColumnBottom(t *testing.T) {
	path := copyFixture(t)
	preBoard, _ := board.Load(path)
	// Record indices of existing todo cards.
	preTodoCount := 0
	for _, c := range preBoard.Cards {
		if c.Column == "todo" {
			preTodoCount++
		}
	}
	cmd := newDummyAddForPath(path, false)
	stdout, _, err := executeCobraText(cmd, []string{"Last todo", "--column=todo"}, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	id := strings.TrimSpace(stdout)
	postBoard, _ := board.Load(path)
	// Find the index of the new card and the index of the last pre-existing todo card.
	var newIdx, lastPreTodoIdx int = -1, -1
	for i, c := range postBoard.Cards {
		if c.ID == id {
			newIdx = i
		} else if c.Column == "todo" {
			if i > lastPreTodoIdx {
				lastPreTodoIdx = i
			}
		}
	}
	if newIdx == -1 {
		t.Fatalf("new card not found in post board")
	}
	if newIdx != lastPreTodoIdx+1 {
		t.Errorf("new card at index %d, want %d (after last pre-existing todo)", newIdx, lastPreTodoIdx+1)
	}
}

func TestAdd_JSONEchoesCard(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyAddForPath(path, true)
	stdout, _, err := executeCobraText(cmd, []string{"Refactor auth", "--column=todo", "--priority=high", "--tags=security,tech-debt", "--description=JWT migration"}, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	var raw struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if raw.Card["title"] != "Refactor auth" {
		t.Errorf("title: %v", raw.Card["title"])
	}
	if raw.Card["column"] != "todo" {
		t.Errorf("column: %v", raw.Card["column"])
	}
	if raw.Card["description"] != "JWT migration" {
		t.Errorf("description: %v", raw.Card["description"])
	}
	if raw.Card["priority"] != "high" {
		t.Errorf("priority: %v", raw.Card["priority"])
	}
	tags := raw.Card["tags"].([]any)
	if len(tags) != 2 || tags[0] != "security" || tags[1] != "tech-debt" {
		t.Errorf("tags: %v", tags)
	}
}

func TestAdd_TextOutputIsIDOnly(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyAddForPath(path, false)
	stdout, stderr, err := executeCobraText(cmd, []string{"Title", "--column=todo"}, false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if stderr != "" {
		t.Errorf("stderr non-empty: %q", stderr)
	}
	// stdout should be exactly <id>\n: 6-char id + newline = 7 bytes.
	if len(stdout) != 7 {
		t.Errorf("stdout len = %d, want 7; got %q", len(stdout), stdout)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Errorf("stdout missing newline: %q", stdout)
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"empty input", "", []string{}, false},
		{"single tag", "security", []string{"security"}, false},
		{"two tags", "a,b", []string{"a", "b"}, false},
		{"surrounding whitespace", "  a , b  ", []string{"a", "b"}, false},
		{"leading comma", ",a", nil, true},
		{"trailing comma", "a,", nil, true},
		{"double comma", "a,,b", nil, true},
		{"only spaces between commas", "a, ,b", nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTags(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (result=%v)", got)
				}
				var ite *InvalidTagError
				if !errors.As(err, &ite) {
					t.Errorf("got %T, want *InvalidTagError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len: got %d want %d (%v vs %v)", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d]: got %q want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
