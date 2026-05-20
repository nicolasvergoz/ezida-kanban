package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

func runPrioritiesCmd(t *testing.T, path string, asJSON bool, args ...string) (string, string, error) {
	t.Helper()
	parent := &cobra.Command{Use: "priorities"}
	add := &cobra.Command{
		Use:  "add",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrioritiesAdd(cmd, path, args[0], asJSON)
		},
	}
	rename := &cobra.Command{
		Use:  "rename",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrioritiesRename(cmd, path, args[0], args[1], asJSON)
		},
	}
	rm := &cobra.Command{
		Use:  "rm",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrioritiesRm(cmd, path, args[0], asJSON)
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

// TestPrioritiesAdd_NoPositionFlag covers task 5.1: --position must be
// rejected by cobra as an unknown flag.
func TestPrioritiesAdd_NoPositionFlag(t *testing.T) {
	jsonFlag := false
	parent := NewPrioritiesCmd(&jsonFlag)
	buf := &bytes.Buffer{}
	parent.SetOut(buf)
	parent.SetErr(buf)
	parent.SetArgs([]string{"add", "urgent", "--position=1"})
	err := parent.Execute()
	if err == nil {
		t.Fatal("expected unknown-flag error")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("got %v, want unknown-flag error", err)
	}
}

func TestPrioritiesAdd_Append(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runPrioritiesCmd(t, path, false, "add", "urgent")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	b, _ := board.Load(path)
	if got := b.Board.Priorities; got[len(got)-1] != "urgent" {
		t.Errorf("priorities: %v", got)
	}
}

func TestPrioritiesAdd_Duplicate(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runPrioritiesCmd(t, path, false, "add", "low")
	if err == nil {
		t.Fatal("expected error")
	}
	var d *DuplicateError
	if !errors.As(err, &d) {
		t.Errorf("got %T", err)
	}
}

func TestPrioritiesRename_Propagates(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runPrioritiesCmd(t, path, false, "rename", "medium", "normal")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	b, _ := board.Load(path)
	found := false
	for _, p := range b.Board.Priorities {
		if p == "normal" {
			found = true
		}
		if p == "medium" {
			t.Errorf("medium still present: %v", b.Board.Priorities)
		}
	}
	if !found {
		t.Errorf("normal not added: %v", b.Board.Priorities)
	}
	for _, c := range b.Cards {
		if c.Priority == "medium" {
			t.Errorf("card %s still references medium", c.ID)
		}
	}
}

func TestPrioritiesRm_InUse(t *testing.T) {
	path := copyFixture(t)
	_, _, err := runPrioritiesCmd(t, path, false, "rm", "high")
	if err == nil {
		t.Fatal("expected error")
	}
	var pe *PriorityInUseError
	if !errors.As(err, &pe) {
		t.Fatalf("got %T", err)
	}
	if pe.Name != "high" {
		t.Errorf("name: %s", pe.Name)
	}
	if len(pe.Cards) == 0 {
		t.Errorf("no cards reported")
	}
}

func TestPrioritiesRm_LastPriority(t *testing.T) {
	path := copyFixture(t)
	// Clear all card priorities so we can remove every priority safely.
	b, _ := board.Load(path)
	for i := range b.Cards {
		b.Cards[i].Priority = ""
	}
	// Reduce priorities to one entry directly via Save to avoid going
	// through priorities-rm three times (which is also valid but more
	// verbose).
	b.Board.Priorities = []string{"low"}
	if err := board.Save(path, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	_, _, err := runPrioritiesCmd(t, path, false, "rm", "low")
	if err == nil {
		t.Fatal("expected error")
	}
	var lp *LastPriorityError
	if !errors.As(err, &lp) {
		t.Errorf("got %T", err)
	}
}

func TestPrioritiesRm_IgnoresCardsWithoutPriority(t *testing.T) {
	path := copyFixture(t)
	// Clear card priorities for any card that uses "low" — there is at
	// least one in the fixture. Then verify "low" can be removed.
	b, _ := board.Load(path)
	for i := range b.Cards {
		if b.Cards[i].Priority == "low" {
			b.Cards[i].Priority = ""
		}
	}
	if err := board.Save(path, b); err != nil {
		t.Fatalf("save: %v", err)
	}
	_, _, err := runPrioritiesCmd(t, path, false, "rm", "low")
	if err != nil {
		t.Fatalf("rm low: %v", err)
	}
	b2, _ := board.Load(path)
	for _, p := range b2.Board.Priorities {
		if p == "low" {
			t.Errorf("low still present: %v", b2.Board.Priorities)
		}
	}
}
