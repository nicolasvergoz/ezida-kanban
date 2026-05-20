package commands

import (
	"testing"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

func TestFixture_Populated_Loads(t *testing.T) {
	b, err := board.Load("testdata/populated.toml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	counts := map[string]int{}
	for _, c := range b.Cards {
		counts[c.Column]++
	}
	want := map[string]int{"todo": 3, "ongoing": 1, "done": 7}
	for col, n := range want {
		if counts[col] != n {
			t.Errorf("column %s: got %d, want %d", col, counts[col], n)
		}
	}
}
