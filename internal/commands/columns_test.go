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

// runColumnsCmd builds the columns parent command pointed at the given
// path. The parent command's subcommands close over the supplied path
// instead of the hard-coded BoardPath.
func runColumnsCmd(t *testing.T, path string, asJSON bool, args ...string) (string, string, error) {
	t.Helper()
	parent := &cobra.Command{Use: "columns"}
	var position int
	add := &cobra.Command{
		Use:  "add",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsAdd(cmd, path, args[0], position, asJSON)
		},
	}
	add.Flags().IntVar(&position, "position", 0, "")
	rename := &cobra.Command{
		Use:  "rename",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsRename(cmd, path, args[0], args[1], asJSON)
		},
	}
	rm := &cobra.Command{
		Use:  "rm",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColumnsRm(cmd, path, args[0], asJSON)
		},
	}
	parent.AddCommand(add, rename, rm)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	parent.SetOut(stdout)
	parent.SetErr(stderr)
	parent.SetArgs(args)
	err := parent.Execute()
	return stdout.String(), stderr.String(), err
}

func TestColumnsHelpListsSubcommands(t *testing.T) {
	jsonFlag := false
	cmd := NewColumnsCmd(&jsonFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	_ = cmd.Execute()
	out := buf.String()
	for _, sub := range []string{"add", "rename", "rm"} {
		if !strings.Contains(out, sub) {
			t.Errorf("missing %s in help:\n%s", sub, out)
		}
	}
}

func TestColumnsAdd_Append(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "add", "review")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	b, _ := board.Load(path)
	if got := b.Board.Columns; got[len(got)-1] != "review" {
		t.Errorf("columns: %v", got)
	}
}

func TestColumnsAdd_AtPosition(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "add", "backlog", "--position=1")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	b, _ := board.Load(path)
	if b.Board.Columns[0] != "backlog" {
		t.Errorf("columns: %v", b.Board.Columns)
	}
}

func TestColumnsAdd_Duplicate(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "add", "todo")
	if err == nil {
		t.Fatal("expected error")
	}
	var d *DuplicateError
	if !errors.As(err, &d) {
		t.Errorf("got %T", err)
	}
}

func TestColumnsAdd_PositionOutOfRange(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "add", "review", "--position=99")
	if err == nil {
		t.Fatal("expected error")
	}
	var p *PositionOutOfRangeError
	if !errors.As(err, &p) {
		t.Errorf("got %T", err)
	}
}

func TestColumnsRename_Propagates(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "rename", "todo", "backlog")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	b, _ := board.Load(path)
	if b.Board.Columns[0] != "backlog" {
		t.Errorf("columns[0]: %s", b.Board.Columns[0])
	}
	for _, c := range b.Cards {
		if c.Column == "todo" {
			t.Errorf("card %s still references todo", c.ID)
		}
	}
}

func TestColumnsRename_UnknownOld(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "rename", "ghost", "anywhere")
	if err == nil {
		t.Fatal("expected error")
	}
	var u *ColumnNotFoundError
	if !errors.As(err, &u) {
		t.Errorf("got %T", err)
	}
}

func TestColumnsRename_DuplicateNew(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "rename", "todo", "done")
	if err == nil {
		t.Fatal("expected error")
	}
	var d *DuplicateError
	if !errors.As(err, &d) {
		t.Errorf("got %T", err)
	}
}

func TestColumnsRm_Unused(t *testing.T) {
	path := copyFixture(t)
	// Add an unused column first.
	if _, _, err := runColumnsCmd(t, path, false, "add", "review"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runColumnsCmd(t, path, false, "rm", "review"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	b, _ := board.Load(path)
	for _, c := range b.Board.Columns {
		if c == "review" {
			t.Errorf("column still present: %v", b.Board.Columns)
		}
	}
}

func TestColumnsRm_InUse_TextOutput(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, false, "rm", "todo")
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *ColumnInUseError
	if !errors.As(err, &ce) {
		t.Fatalf("got %T", err)
	}
	if ce.Name != "todo" {
		t.Errorf("name: %s", ce.Name)
	}
	if len(ce.Cards) != 3 {
		t.Errorf("cards: %d (want 3)", len(ce.Cards))
	}
	// Verify the text rendering includes the indented card lines.
	text := ce.Error()
	if !strings.Contains(text, "Move or remove these cards first.") {
		t.Errorf("missing trailing sentence: %s", text)
	}
	for _, c := range ce.Cards {
		want := "  " + c.ID + "  " + c.Title
		if !strings.Contains(text, want) {
			t.Errorf("missing line %q in:\n%s", want, text)
		}
	}
}

func TestColumnsRm_InUse_JSONOutput(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runColumnsCmd(t, path, true, "rm", "todo")
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *ColumnInUseError
	if !errors.As(err, &ce) {
		t.Fatalf("got %T", err)
	}
	// The output package's Fail layer produces the JSON envelope. Here
	// we sanity-check the Details() payload directly: it's a map with
	// "column" + "cards" where cards is a []affectedCard.
	det := ce.Details().(map[string]any)
	if det["column"] != "todo" {
		t.Errorf("column: %v", det["column"])
	}
	cards := det["cards"].([]affectedCard)
	if len(cards) != 3 {
		t.Errorf("cards: %d", len(cards))
	}
	// Round-trip via the output envelope to confirm the shape.
	buf, _ := json.Marshal(det)
	var raw map[string]any
	if err := json.Unmarshal(buf, &raw); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf)
	}
	jsonCards := raw["cards"].([]any)
	if len(jsonCards) != 3 {
		t.Errorf("json cards: %d", len(jsonCards))
	}
	first := jsonCards[0].(map[string]any)
	for _, k := range []string{"id", "title"} {
		if _, has := first[k]; !has {
			t.Errorf("missing key %s in %v", k, first)
		}
	}
}

func TestColumnsRm_LastColumn(t *testing.T) {
	path := copyFixture(t)
	// Remove every column except one by repeatedly renaming/removing.
	// Easiest path: rename todo so it has no referencing cards, then
	// remove the others. Simpler: edit the file directly via a save
	// with a single-column board (already validated empty list rejected
	// by validation).
	b, _ := board.Load(path)
	// Move all cards into "todo" so we can remove "ongoing" and "done".
	for i := range b.Cards {
		b.Cards[i].Column = "todo"
	}
	if err := board.Save(path, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, _, err := runColumnsCmd(t, path, false, "rm", "ongoing"); err != nil {
		t.Fatalf("rm ongoing: %v", err)
	}
	if _, _, err := runColumnsCmd(t, path, false, "rm", "done"); err != nil {
		t.Fatalf("rm done: %v", err)
	}
	// Now move all cards out — wait, no other column exists. Instead
	// drop the cards first:
	b, _ = board.Load(path)
	b.Cards = nil
	if err := board.Save(path, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Now try to remove the last column.
	_, _, err := runColumnsCmd(t, path, false, "rm", "todo")
	if err == nil {
		t.Fatal("expected error")
	}
	var lc *LastColumnError
	if !errors.As(err, &lc) {
		t.Errorf("got %T", err)
	}
}
