package board

import (
	"fmt"
	"strings"
)

// PaletteColor is one named entry of EpicPalette.
type PaletteColor struct {
	Name string
	Hex  string
}

// EpicPalette is the ordered set of named colors offered to epics.
//
// The order is chromatic distance, not hue: two epics created back to
// back must stay distinguishable at chip size, which ordering by hue
// (violet → fuchsia → pink) would not achieve.
//
// No entry may collide with DefaultPriorityColors, so a colored epic
// chip is never read as a priority signal. A test enforces that.
var EpicPalette = []PaletteColor{
	{Name: "violet", Hex: "#8b5cf6"},
	{Name: "emerald", Hex: "#10b981"},
	{Name: "orange", Hex: "#f97316"},
	{Name: "blue", Hex: "#3b82f6"},
	{Name: "pink", Hex: "#ec4899"},
	{Name: "lime", Hex: "#84cc16"},
	{Name: "cyan", Hex: "#06b6d4"},
	{Name: "fuchsia", Hex: "#d946ef"},
}

// InvalidColorError is returned by ResolveColor when the input is
// neither a palette name nor a well-formed hex value. The CLI maps it
// to the wire code INVALID_COLOR.
type InvalidColorError struct {
	Value string
}

func (e *InvalidColorError) Error() string {
	return fmt.Sprintf(
		"board: color %q is neither a palette name nor a hex value like #rgb or #rrggbb",
		e.Value)
}

// PaletteNames returns the palette names in order, for help text and
// error messages.
func PaletteNames() []string {
	out := make([]string, 0, len(EpicPalette))
	for _, p := range EpicPalette {
		out = append(out, p.Name)
	}
	return out
}

// ResolveColor maps a palette name to its hex value, or validates a
// literal hex and returns it unchanged. Only hex ever reaches the file,
// so a consumer that has never heard of the palette still reads a valid
// color (design D4).
func ResolveColor(nameOrHex string) (string, error) {
	v := strings.TrimSpace(nameOrHex)
	for _, p := range EpicPalette {
		if p.Name == v {
			return p.Hex, nil
		}
	}
	if hexColorPattern.MatchString(v) {
		return v, nil
	}
	return "", &InvalidColorError{Value: nameOrHex}
}

// ColorName returns the palette name bound to hex, or "" when the value
// is off-palette.
func ColorName(hex string) string {
	for _, p := range EpicPalette {
		if p.Hex == hex {
			return p.Name
		}
	}
	return ""
}

// AssignColor returns the palette color least used by the cards on b,
// breaking ties by palette order.
//
// Least-used rather than next-in-sequence: a modulo-of-count strategy
// hands out a color already in use as soon as an epic is deleted
// (design D5). Off-palette colors are counted too, so a hand-assigned
// value still pushes the palette away from it.
func AssignColor(b *Board) string {
	used := make(map[string]int, len(EpicPalette))
	for _, c := range b.Cards {
		if c.Color != "" {
			used[c.Color]++
		}
	}
	best := EpicPalette[0]
	bestCount := used[best.Hex]
	for _, p := range EpicPalette[1:] {
		if n := used[p.Hex]; n < bestCount {
			best, bestCount = p, n
		}
	}
	return best.Hex
}
