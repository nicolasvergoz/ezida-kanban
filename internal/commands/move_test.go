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

func newDummyMoveForPath(path string, asJSON bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "move",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMove(cmd, path, args[0], args[1], asJSON)
		},
	}
	return cmd
}

func TestMove_Help(t *testing.T) {
	jsonFlag := false
	cmd := NewMoveCmd(&jsonFlag)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
}

func TestMove_HappyPath(t *testing.T) {
	path := copyFixture(t)
	preBoard, _ := board.Load(path)
	var origCard board.Card
	for _, c := range preBoard.Cards {
		if c.Column == "todo" {
			origCard = c
			break
		}
	}
	cmd := newDummyMoveForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{origCard.ID, "ongoing"}, false)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	postBoard, _ := board.Load(path)
	var moved *board.Card
	for i := range postBoard.Cards {
		if postBoard.Cards[i].ID == origCard.ID {
			moved = &postBoard.Cards[i]
		}
	}
	if moved == nil {
		t.Fatalf("card disappeared")
	}
	if moved.Column != "ongoing" {
		t.Errorf("column: got %q, want ongoing", moved.Column)
	}
	if !moved.UpdatedAt.After(origCard.CreatedAt) && !moved.UpdatedAt.Equal(origCard.CreatedAt) {
		// updated_at must be >= created_at (validation rule 9 enforces this; we
		// additionally want it refreshed).
		t.Errorf("updated_at not refreshed: %v vs created %v", moved.UpdatedAt, origCard.CreatedAt)
	}
}

func TestMove_SameColumn(t *testing.T) {
	path := copyFixture(t)
	preBoard, _ := board.Load(path)
	var orig board.Card
	for _, c := range preBoard.Cards {
		if c.Column == "todo" {
			orig = c
			break
		}
	}
	cmd := newDummyMoveForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{orig.ID, "todo"}, false)
	if err != nil {
		t.Fatalf("move same column: %v", err)
	}
	postBoard, _ := board.Load(path)
	var moved *board.Card
	for i := range postBoard.Cards {
		if postBoard.Cards[i].ID == orig.ID {
			moved = &postBoard.Cards[i]
		}
	}
	if moved == nil {
		t.Fatalf("card disappeared")
	}
	if moved.Column != "todo" {
		t.Errorf("column changed: %s", moved.Column)
	}
}

func TestMove_UnknownColumn(t *testing.T) {
	path := copyFixture(t)
	pre, _ := os.ReadFile(path)
	cmd := newDummyMoveForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"a3f2k9", "ghost"}, false)
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

func TestMove_UnknownCard(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyMoveForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{"zzzzzz", "todo"}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	var cnf *CardNotFoundError
	if !errors.As(err, &cnf) {
		t.Errorf("got %T, want *CardNotFoundError", err)
	}
}

func TestMove_JSONEchoesCard(t *testing.T) {
	path := copyFixture(t)
	cmd := newDummyMoveForPath(path, true)
	stdout, _, err := executeCobraText(cmd, []string{"a3f2k9", "ongoing"}, false)
	if err != nil {
		t.Fatalf("move: %v", err)
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
	if raw.Card["column"] != "ongoing" {
		t.Errorf("column: %v", raw.Card["column"])
	}
}

func TestMove_PreservesOtherCardsOrder(t *testing.T) {
	path := copyFixture(t)
	preBoard, _ := board.Load(path)
	// Pick the first todo card to move.
	var moverID string
	for _, c := range preBoard.Cards {
		if c.Column == "todo" {
			moverID = c.ID
			break
		}
	}
	// Record IDs of all cards in pre-board, in order, excluding the mover.
	var preIDsExcl []string
	for _, c := range preBoard.Cards {
		if c.ID != moverID {
			preIDsExcl = append(preIDsExcl, c.ID)
		}
	}

	cmd := newDummyMoveForPath(path, false)
	_, _, err := executeCobraText(cmd, []string{moverID, "ongoing"}, false)
	if err != nil {
		t.Fatalf("move: %v", err)
	}

	postBoard, _ := board.Load(path)
	var postIDsExcl []string
	for _, c := range postBoard.Cards {
		if c.ID != moverID {
			postIDsExcl = append(postIDsExcl, c.ID)
		}
	}

	if strings.Join(preIDsExcl, ",") != strings.Join(postIDsExcl, ",") {
		t.Errorf("other cards reordered:\nbefore: %v\nafter:  %v", preIDsExcl, postIDsExcl)
	}
}
