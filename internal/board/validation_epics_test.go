package board

import (
	"testing"
	"time"
)

// ruleBoard builds a minimal valid board and hands it to mutate, so
// each case below differs from a passing board by exactly one thing.
func ruleBoard(mutate func(b *Board)) *Board {
	now := time.Date(2026, 5, 20, 14, 30, 0, 0, time.UTC)
	b := &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    []string{"todo", "done"},
			Priorities: []string{"low"},
		},
		Cards: []Card{
			{ID: "rl4m9x", Title: "parent", Column: "todo", Tags: []string{}, CreatedAt: now, UpdatedAt: now},
			{ID: "f20wbo", Title: "child", Column: "todo", Tags: []string{}, CreatedAt: now, UpdatedAt: now},
			{ID: "a3f2k9", Title: "loose", Column: "done", Tags: []string{}, CreatedAt: now, UpdatedAt: now},
		},
	}
	b.Board.SetDoneColumn("done", true)
	if mutate != nil {
		mutate(b)
	}
	return b
}

func TestValidate_EpicRules(t *testing.T) {
	cases := []struct {
		name string
		rule int
		mut  func(b *Board)
	}{
		{
			name: "rule 11: epic names no card",
			rule: 11,
			mut:  func(b *Board) { b.Cards[1].Epic = "zzzzzz" },
		},
		{
			name: "rule 12: epic is the card itself",
			rule: 12,
			mut:  func(b *Board) { b.Cards[1].Epic = "f20wbo" },
		},
		{
			// A → B → C: B carries an epic and is named as one.
			name: "rule 13: two-level chain",
			rule: 13,
			mut: func(b *Board) {
				b.Cards[1].Epic = "rl4m9x"
				b.Cards[2].Epic = "f20wbo"
			},
		},
		{
			name: "rule 14: color is not hex",
			rule: 14,
			mut:  func(b *Board) { b.Cards[0].Color = "violet" },
		},
		{
			name: "rule 14: truncated hex",
			rule: 14,
			mut:  func(b *Board) { b.Cards[0].Color = "#12" },
		},
		{
			name: "rule 15: epic on a pre-v2 board",
			rule: 15,
			mut: func(b *Board) {
				b.SchemaVersion = 1
				b.Cards[1].Epic = "rl4m9x"
			},
		},
		{
			name: "rule 16: reserved character in a column name",
			rule: 16,
			mut: func(b *Board) {
				b.Board.Columns = []string{"todo", "do*ne"}
				b.Cards[2].Column = "todo"
			},
		},
		{
			name: "rule 16: empty column name",
			rule: 16,
			mut: func(b *Board) {
				b.Board.Columns = []string{"todo", ""}
				b.Cards[2].Column = "todo"
			},
		},
		{
			name: "rule 17: two columns decode to the same name",
			rule: 17,
			mut: func(b *Board) {
				b.Board.Columns = []string{"todo", "done", "done"}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verr := Validate(ruleBoard(tc.mut))
			if verr == nil || !hasViolationForRule(verr, tc.rule) {
				t.Fatalf("expected rule %d violation, got %v", tc.rule, verr)
			}
		})
	}
}

// TestValidate_EpicsValidBoard exercises every new rule at once on a
// board that satisfies all of them.
func TestValidate_EpicsValidBoard(t *testing.T) {
	b := ruleBoard(func(b *Board) {
		b.Board.Columns = []string{"todo", "done", "wont-fix"}
		b.Board.SetDoneColumn("wont-fix", true)
		b.Cards[0].Color = "#8b5cf6"
		b.Cards[1].Epic = "rl4m9x"
		b.Cards[2].Epic = "rl4m9x"
	})
	if verr := Validate(b); verr != nil {
		t.Fatalf("valid board rejected: %v", verr)
	}
}

// Many children on one parent is the shape this feature exists for.
func TestValidate_ManyChildrenOneParentIsLegal(t *testing.T) {
	b := ruleBoard(func(b *Board) {
		now := b.Cards[0].CreatedAt
		for _, id := range []string{"c00001", "c00002", "c00003", "c00004"} {
			b.Cards = append(b.Cards, Card{
				ID: id, Title: "child", Column: "todo", Tags: []string{},
				Epic: "rl4m9x", CreatedAt: now, UpdatedAt: now,
			})
		}
	})
	if verr := Validate(b); verr != nil {
		t.Fatalf("board with four children rejected: %v", verr)
	}
}

// A card may carry a color without having children: the color is what
// a parent lends its children, and a card acquires it before or after
// the first child arrives.
func TestValidate_ColorWithoutChildrenIsLegal(t *testing.T) {
	b := ruleBoard(func(b *Board) { b.Cards[2].Color = "#06b6d4" })
	if verr := Validate(b); verr != nil {
		t.Fatalf("childless colored card rejected: %v", verr)
	}
}

// A board declaring no terminal column at all is legal.
func TestValidate_NoTerminalColumnIsLegal(t *testing.T) {
	b := ruleBoard(func(b *Board) { b.Board.SetDoneColumn("done", false) })
	if verr := Validate(b); verr != nil {
		t.Fatalf("board with no terminal column rejected: %v", verr)
	}
	if got := b.Board.DoneColumns(); len(got) != 0 {
		t.Fatalf("DoneColumns() = %v, want empty", got)
	}
}
