package board

import "testing"

// colorBoard builds a board whose cards carry the given colors, one
// card per color, so AssignColor sees the intended histogram.
func colorBoard(colors ...string) *Board {
	b := &Board{
		SchemaVersion: SupportedSchemaVersion,
		Board: BoardConfig{
			Columns:    []string{"todo"},
			Priorities: []string{"low"},
		},
	}
	for i, hex := range colors {
		b.Cards = append(b.Cards, Card{
			ID:     string(rune('a'+i)) + "00000",
			Title:  "card",
			Column: "todo",
			Color:  hex,
		})
	}
	return b
}

func TestAssignColor(t *testing.T) {
	cases := []struct {
		name  string
		board *Board
		want  string
	}{
		{
			name:  "empty board takes the first palette entry",
			board: colorBoard(),
			want:  "#8b5cf6",
		},
		{
			name:  "partially used palette takes the first unused entry",
			board: colorBoard("#8b5cf6", "#10b981"),
			want:  "#f97316",
		},
		{
			// Every entry used once: the tie breaks by palette order.
			name: "exhausted palette reuses the earliest least-used",
			board: colorBoard("#8b5cf6", "#10b981", "#f97316", "#3b82f6",
				"#ec4899", "#84cc16", "#06b6d4", "#d946ef"),
			want: "#8b5cf6",
		},
		{
			// The case a modulo-of-count strategy gets wrong: three
			// epics created, the middle one deleted, a fourth created.
			// A counter would hand out the palette's 4th entry even
			// though the 2nd is free.
			name:  "deletion frees its color for the next assignment",
			board: colorBoard("#8b5cf6", "#f97316"),
			want:  "#10b981",
		},
		{
			// An off-palette color counts toward nothing in the
			// palette, so the first entry is still free.
			name:  "off-palette colors do not consume palette entries",
			board: colorBoard("#7c3aed"),
			want:  "#8b5cf6",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AssignColor(tc.board); got != tc.want {
				t.Fatalf("AssignColor = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestEpicPalette_DisjointFromPriorityColors keeps an epic chip from
// ever being mistaken for a priority indicator.
func TestEpicPalette_DisjointFromPriorityColors(t *testing.T) {
	for _, p := range EpicPalette {
		for name, hex := range DefaultPriorityColors {
			if p.Hex == hex {
				t.Errorf("palette entry %s (%s) collides with priority %q", p.Name, p.Hex, name)
			}
		}
	}
}

func TestEpicPalette_HexValuesAreValid(t *testing.T) {
	seen := make(map[string]struct{}, len(EpicPalette))
	for _, p := range EpicPalette {
		if !hexColorPattern.MatchString(p.Hex) {
			t.Errorf("palette entry %s has malformed hex %q", p.Name, p.Hex)
		}
		if _, dup := seen[p.Hex]; dup {
			t.Errorf("palette contains %s twice", p.Hex)
		}
		seen[p.Hex] = struct{}{}
	}
}

func TestResolveColor(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "violet", want: "#8b5cf6"},
		{in: "emerald", want: "#10b981"},
		{in: "#7c3aed", want: "#7c3aed"},
		{in: "#abc", want: "#abc"},
		{in: "chartreuse", wantErr: true},
		{in: "#12", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		got, err := ResolveColor(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ResolveColor(%q) = %q, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ResolveColor(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ResolveColor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestColorName(t *testing.T) {
	if got := ColorName("#10b981"); got != "emerald" {
		t.Errorf("ColorName(#10b981) = %q, want emerald", got)
	}
	if got := ColorName("#7c3aed"); got != "" {
		t.Errorf("ColorName(off-palette) = %q, want empty", got)
	}
}
