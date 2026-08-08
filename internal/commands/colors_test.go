package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

func newDummyColorsForPath(path string, asJSON bool) *cobra.Command {
	return &cobra.Command{
		Use:  "colors",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColors(cmd, path, asJSON)
		},
	}
}

type colorEntry struct {
	Name   *string `json:"name"`
	Hex    string  `json:"hex"`
	HeldBy *struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"held_by"`
}

func colorsJSON(t *testing.T, path string) []colorEntry {
	t.Helper()
	stdout, _, err := executeCobraText(newDummyColorsForPath(path, true), nil, true)
	if err != nil {
		t.Fatalf("colors: %v", err)
	}
	var env struct {
		Colors []colorEntry `json:"colors"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("not JSON: %v (%q)", err, stdout)
	}
	return env.Colors
}

func TestColors_ReportsHoldersAndFreeEntries(t *testing.T) {
	path := copyEpicsFixture(t) // rl4m9x holds #8b5cf6 (violet)
	entries := colorsJSON(t, path)
	if len(entries) != len(board.EpicPalette) {
		t.Fatalf("got %d entries, want %d", len(entries), len(board.EpicPalette))
	}
	for _, e := range entries {
		if e.Name == nil {
			t.Fatalf("palette entry %s has a null name", e.Hex)
		}
		switch *e.Name {
		case "violet":
			if e.HeldBy == nil || e.HeldBy.ID != "rl4m9x" {
				t.Errorf("violet held_by = %v, want rl4m9x", e.HeldBy)
			}
			if e.HeldBy.Title != "Card relations" {
				t.Errorf("violet holder title = %q", e.HeldBy.Title)
			}
		default:
			if e.HeldBy != nil {
				t.Errorf("%s should be free, held by %v", *e.Name, e.HeldBy)
			}
		}
	}
}

// A hand-assigned color the palette has never heard of is still held;
// omitting it would advertise a slot that is visually taken.
func TestColors_IncludesOffPaletteColorsInUse(t *testing.T) {
	path := copyEpicsFixture(t)
	b := loadBoard(t, path)
	b.Cards[2].Color = "#7c3aed"
	if err := board.Save(path, b); err != nil {
		t.Fatalf("seed: %v", err)
	}

	entries := colorsJSON(t, path)
	if len(entries) != len(board.EpicPalette)+1 {
		t.Fatalf("got %d entries, want %d", len(entries), len(board.EpicPalette)+1)
	}
	extra := entries[len(entries)-1]
	if extra.Name != nil {
		t.Errorf("off-palette entry name = %v, want null", *extra.Name)
	}
	if extra.Hex != "#7c3aed" {
		t.Errorf("off-palette hex = %s", extra.Hex)
	}
	if extra.HeldBy == nil || extra.HeldBy.ID != "a3f2k9" {
		t.Errorf("off-palette held_by = %v", extra.HeldBy)
	}
}

func TestColors_DoesNotMutateTheBoard(t *testing.T) {
	path := copyEpicsFixture(t)
	before := readFile(t, path)
	if _, _, err := executeCobraText(newDummyColorsForPath(path, false), nil, false); err != nil {
		t.Fatalf("colors: %v", err)
	}
	if readFile(t, path) != before {
		t.Error("`colors` wrote to kanban.toml")
	}
}

func TestColors_TextModeDistinguishesHeldFromFree(t *testing.T) {
	path := copyEpicsFixture(t)
	stdout, _, err := executeCobraText(newDummyColorsForPath(path, false), nil, false)
	if err != nil {
		t.Fatalf("colors: %v", err)
	}
	if !strings.Contains(stdout, "violet") || !strings.Contains(stdout, "rl4m9x") {
		t.Errorf("held entry not reported:\n%s", stdout)
	}
	if !strings.Contains(stdout, "free") {
		t.Errorf("free entries not reported:\n%s", stdout)
	}
}
