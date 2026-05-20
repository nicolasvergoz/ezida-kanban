package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// newDummyEditForPath builds an edit command that writes to the given
// absolute board path, mirroring NewEditCmd's flag wiring but routing
// through the testable runEdit.
func newDummyEditForPath(path string, asJSON bool) *cobra.Command {
	state := editFlagState{}
	cmd := &cobra.Command{
		Use:  "edit",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := buildEditFlags(cmd, state)
			return runEdit(cmd, path, args[0], f, asJSON)
		},
	}
	cmd.Flags().StringVar(&state.title, "title", "", "")
	cmd.Flags().StringVar(&state.description, "description", "", "")
	cmd.Flags().StringVar(&state.priority, "priority", "", "")
	cmd.Flags().StringVar(&state.tags, "tags", "", "")
	cmd.Flags().StringVar(&state.column, "column", "", "")
	return cmd
}

func TestEdit_HelpListsAllFlags(t *testing.T) {
	jsonFlag := false
	cmd := NewEditCmd(&jsonFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	_ = cmd.Execute()
	out := buf.String()
	for _, flag := range []string{"--title", "--description", "--priority", "--tags", "--column"} {
		if !strings.Contains(out, flag) {
			t.Errorf("help missing %s flag:\n%s", flag, out)
		}
	}
}

// TestEdit_BuildFlags_Pointers covers task 2.2.
func TestEdit_BuildFlags_Pointers(t *testing.T) {
	t.Run("unset flags resolve to nil", func(t *testing.T) {
		cmd := newDummyEditForPath("/tmp/ignored", false)
		// Parse with no args — no flag changed.
		_ = cmd.ParseFlags([]string{})
		// Reach into the same builder.
		state := editFlagState{}
		f := buildEditFlags(cmd, state)
		if f.title != nil || f.description != nil || f.priority != nil || f.tags != nil || f.column != nil {
			t.Errorf("expected all nil, got %+v", f)
		}
	})
	t.Run("set-but-empty flag resolves to non-nil pointer to \"\"", func(t *testing.T) {
		state := editFlagState{}
		cmd := &cobra.Command{Use: "x"}
		cmd.Flags().StringVar(&state.priority, "priority", "", "")
		_ = cmd.ParseFlags([]string{"--priority="})
		f := buildEditFlags(cmd, state)
		if f.priority == nil {
			t.Fatal("priority is nil; want non-nil pointer to \"\"")
		}
		if *f.priority != "" {
			t.Errorf("priority value: got %q, want \"\"", *f.priority)
		}
	})
}

func TestEdit_HappyPath(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyEditForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9", "--title=New title"}, false)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	b, _ := board.Load(path)
	var found *board.Card
	for i := range b.Cards {
		if b.Cards[i].ID == "a3f2k9" {
			found = &b.Cards[i]
		}
	}
	if found == nil {
		t.Fatalf("card not found")
	}
	if found.Title != "New title" {
		t.Errorf("title: %q", found.Title)
	}
}

func TestEdit_MultipleFields(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyEditForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{
		"a3f2k9",
		"--title=New",
		"--priority=low",
		"--tags=a,b",
	}, false)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	b, _ := board.Load(path)
	var found *board.Card
	for i := range b.Cards {
		if b.Cards[i].ID == "a3f2k9" {
			found = &b.Cards[i]
		}
	}
	if found.Title != "New" {
		t.Errorf("title: %q", found.Title)
	}
	if found.Priority != "low" {
		t.Errorf("priority: %q", found.Priority)
	}
	if len(found.Tags) != 2 || found.Tags[0] != "a" || found.Tags[1] != "b" {
		t.Errorf("tags: %v", found.Tags)
	}
}

func TestEdit_NoFlags(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyEditForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var nte *NothingToEditError
	if !errors.As(err, &nte) {
		t.Errorf("got %T, want *NothingToEditError", err)
	}
}

func TestEdit_ClearPriority(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyEditForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9", "--priority="}, false)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	b, _ := board.Load(path)
	for _, c := range b.Cards {
		if c.ID == "a3f2k9" {
			if c.Priority != "" {
				t.Errorf("priority not cleared: %q", c.Priority)
			}
			return
		}
	}
	t.Fatal("card not found")
}

func TestEdit_ChangeColumnReOrders(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyEditForPath(path, false)
	// a3f2k9 is in todo. Move it to ongoing — should end up at the end
	// of the ongoing block.
	_, _, err := executeCobraText(cmd, []string{"a3f2k9", "--column=ongoing"}, false)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	b, _ := board.Load(path)
	// Find the index of a3f2k9 and confirm every preceding card with column=ongoing is before it
	// AND no card with column=ongoing exists after it.
	var idx int = -1
	for i, c := range b.Cards {
		if c.ID == "a3f2k9" {
			idx = i
		}
	}
	if idx == -1 {
		t.Fatal("card not found")
	}
	if b.Cards[idx].Column != "ongoing" {
		t.Errorf("column: %q", b.Cards[idx].Column)
	}
	for i := idx + 1; i < len(b.Cards); i++ {
		if b.Cards[i].Column == "ongoing" {
			t.Errorf("ongoing card after the moved one at index %d", i)
		}
	}
}

func TestEdit_InvalidPriority(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyEditForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9", "--priority=urgent"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var ipe *InvalidPriorityError
	if !errors.As(err, &ipe) {
		t.Errorf("got %T, want *InvalidPriorityError", err)
	}
}

func TestEdit_EmptyTitleRejected(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyEditForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9", "--title="}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var mte *MissingTitleError
	if !errors.As(err, &mte) {
		t.Errorf("got %T, want *MissingTitleError", err)
	}
}

func TestEdit_JSONEchoesCard(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyEditForPath(path, true)
	stdout, _, err := executeCobraText(cmd, []string{"a3f2k9", "--title=Edited"}, false)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	var raw struct {
		Card map[string]any `json:"card"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	if raw.Card["id"] != "a3f2k9" {
		t.Errorf("id: %v", raw.Card["id"])
	}
	if raw.Card["title"] != "Edited" {
		t.Errorf("title: %v", raw.Card["title"])
	}
}
