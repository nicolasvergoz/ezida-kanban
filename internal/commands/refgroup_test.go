package commands

import (
	"errors"
	"testing"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// newColumnsGroup constructs a refGroup wired for columns. Mirrors the
// production constructor used by NewColumnsCmd. Kept here so refgroup
// tests do not need the cobra layer.
func newColumnsGroup(b *board.Board) *refGroup {
	return &refGroup{
		list:          &b.Board.Columns,
		cardField:     func(c *board.Card) *string { return &c.Column },
		isReferencing: func(v, n string) bool { return v == n },
		inUseErr: func(name string, cards []affectedCard) error {
			return &ColumnInUseError{Name: name, Cards: cards}
		},
		lastErr:      func(name string) error { return &LastColumnError{Name: name} },
		duplicateErr: func(name string) error { return &DuplicateError{Kind: "column", Name: name} },
		unknownErr:   func(name string) error { return &ColumnNotFoundError{Name: name} },
		positionErr:  func(p, m int) error { return &PositionOutOfRangeError{Position: p, Max: m} },
	}
}

func newPrioritiesGroup(b *board.Board) *refGroup {
	return &refGroup{
		list:          &b.Board.Priorities,
		cardField:     func(c *board.Card) *string { return &c.Priority },
		isReferencing: func(v, n string) bool { return v != "" && v == n },
		inUseErr: func(name string, cards []affectedCard) error {
			return &PriorityInUseError{Name: name, Cards: cards}
		},
		lastErr:      func(name string) error { return &LastPriorityError{Name: name} },
		duplicateErr: func(name string) error { return &DuplicateError{Kind: "priority", Name: name} },
		unknownErr:   func(name string) error { return &InvalidPriorityError{Name: name} },
		positionErr:  func(p, m int) error { return &PositionOutOfRangeError{Position: p, Max: m} },
	}
}

func freshBoard() *board.Board {
	return &board.Board{
		SchemaVersion: 1,
		Board: board.BoardConfig{
			Columns:    []string{"todo", "ongoing", "done"},
			Priorities: []string{"low", "medium", "high"},
		},
		Cards: nil,
	}
}

func TestRefGroup_Add_AppendByDefault(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	if err := g.add("review", 0); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := b.Board.Columns; got[len(got)-1] != "review" {
		t.Errorf("columns: %v", got)
	}
}

func TestRefGroup_Add_AtPosition1(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	if err := g.add("backlog", 1); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := b.Board.Columns[0]; got != "backlog" {
		t.Errorf("first: %s", got)
	}
}

func TestRefGroup_Add_AtPositionLenPlus1(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	if err := g.add("review", 4); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := b.Board.Columns[3]; got != "review" {
		t.Errorf("last: %s", got)
	}
}

func TestRefGroup_Add_PositionLenPlus2_OutOfRange(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	err := g.add("review", 5)
	if err == nil {
		t.Fatal("expected error")
	}
	var p *PositionOutOfRangeError
	if !errors.As(err, &p) {
		t.Errorf("got %T", err)
	}
}

func TestRefGroup_Add_PositionZeroAfterMin_OutOfRangeWhenSet(t *testing.T) {
	// position=0 means "append" by our contract. We separately test
	// position=-1 as an explicit out-of-range case.
	b := freshBoard()
	g := newColumnsGroup(b)
	err := g.add("x", -1)
	if err == nil {
		t.Fatal("expected error")
	}
	var p *PositionOutOfRangeError
	if !errors.As(err, &p) {
		t.Errorf("got %T", err)
	}
}

func TestRefGroup_Add_Duplicate(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	err := g.add("todo", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	var d *DuplicateError
	if !errors.As(err, &d) {
		t.Errorf("got %T", err)
	}
}

func TestRefGroup_Rename_ToSelf_NoOp(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	if err := g.rename(b, "todo", "todo"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if b.Board.Columns[0] != "todo" {
		t.Errorf("first: %s", b.Board.Columns[0])
	}
}

func TestRefGroup_Rename_UnusedName(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	if err := g.rename(b, "ongoing", "wip"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if b.Board.Columns[1] != "wip" {
		t.Errorf("second: %s", b.Board.Columns[1])
	}
}

func TestRefGroup_Rename_Propagates(t *testing.T) {
	b := freshBoard()
	b.Cards = []board.Card{
		{ID: "a", Column: "todo", Title: "a"},
		{ID: "b", Column: "todo", Title: "b"},
		{ID: "c", Column: "done", Title: "c"},
	}
	g := newColumnsGroup(b)
	if err := g.rename(b, "todo", "backlog"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if b.Cards[0].Column != "backlog" || b.Cards[1].Column != "backlog" {
		t.Errorf("propagation: %v %v", b.Cards[0].Column, b.Cards[1].Column)
	}
	if b.Cards[2].Column != "done" {
		t.Errorf("unrelated card touched: %v", b.Cards[2].Column)
	}
}

func TestRefGroup_Rename_DuplicateNew(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	err := g.rename(b, "todo", "done")
	if err == nil {
		t.Fatal("expected error")
	}
	var d *DuplicateError
	if !errors.As(err, &d) {
		t.Errorf("got %T", err)
	}
}

func TestRefGroup_Rename_UnknownOld(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	err := g.rename(b, "ghost", "anywhere")
	if err == nil {
		t.Fatal("expected error")
	}
	var u *ColumnNotFoundError
	if !errors.As(err, &u) {
		t.Errorf("got %T", err)
	}
}

func TestRefGroup_Remove_NoViolators(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	if err := g.remove(b, "ongoing"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(b.Board.Columns) != 2 {
		t.Errorf("len: %d", len(b.Board.Columns))
	}
}

func TestRefGroup_Remove_OneViolator(t *testing.T) {
	b := freshBoard()
	b.Cards = []board.Card{{ID: "a3f2k9", Title: "Refactor", Column: "todo"}}
	g := newColumnsGroup(b)
	err := g.remove(b, "todo")
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *ColumnInUseError
	if !errors.As(err, &ce) {
		t.Fatalf("got %T", err)
	}
	if len(ce.Cards) != 1 {
		t.Errorf("cards: %v", ce.Cards)
	}
}

func TestRefGroup_Remove_ManyViolators(t *testing.T) {
	b := freshBoard()
	b.Cards = []board.Card{
		{ID: "a3f2k9", Title: "T1", Column: "todo"},
		{ID: "b7m1p4", Title: "T2", Column: "todo"},
		{ID: "c8q3r5", Title: "T3", Column: "todo"},
	}
	g := newColumnsGroup(b)
	err := g.remove(b, "todo")
	var ce *ColumnInUseError
	if !errors.As(err, &ce) {
		t.Fatalf("got %T", err)
	}
	if len(ce.Cards) != 3 {
		t.Errorf("cards: %d", len(ce.Cards))
	}
}

func TestRefGroup_Remove_LastEntry(t *testing.T) {
	b := freshBoard()
	b.Board.Columns = []string{"only"}
	g := newColumnsGroup(b)
	err := g.remove(b, "only")
	if err == nil {
		t.Fatal("expected error")
	}
	var lc *LastColumnError
	if !errors.As(err, &lc) {
		t.Errorf("got %T", err)
	}
}

func TestRefGroup_Remove_UnknownName(t *testing.T) {
	b := freshBoard()
	g := newColumnsGroup(b)
	err := g.remove(b, "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
	var u *ColumnNotFoundError
	if !errors.As(err, &u) {
		t.Errorf("got %T", err)
	}
}

func TestRefGroup_Priorities_IgnoresCardsWithoutPriority(t *testing.T) {
	b := freshBoard()
	b.Cards = []board.Card{
		// Card with empty priority should NOT count as a violator.
		{ID: "a3f2k9", Title: "x", Column: "todo", Priority: ""},
	}
	g := newPrioritiesGroup(b)
	if err := g.remove(b, "high"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(b.Board.Priorities) != 2 {
		t.Errorf("priorities: %v", b.Board.Priorities)
	}
}
