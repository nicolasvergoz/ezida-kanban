package commands

import (
	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// NewColorsCmd builds the `ezida colors` command.
func NewColorsCmd(jsonOut *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "colors",
		Short: "List the epic color palette and which card holds each entry",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runColors(cmd, BoardPath, *jsonOut)
		},
	}
}

// runColors lists every palette entry with its holder, then any
// off-palette color currently in use. It never mutates the board.
//
// Off-palette entries are included because the file is hand-editable:
// a color the palette has never heard of is still held, and omitting it
// would make `colors` claim a free slot that is visually taken.
func runColors(cmd *cobra.Command, path string, asJSON bool) error {
	b, err := board.Load(path)
	if err != nil {
		return err
	}

	// First holder in board file order wins, matching how the rest of
	// the CLI resolves ties.
	holders := make(map[string]*output.ColorHolder, len(b.Cards))
	var offPalette []string
	for i := range b.Cards {
		c := b.Cards[i]
		if c.Color == "" {
			continue
		}
		if _, seen := holders[c.Color]; !seen {
			holders[c.Color] = &output.ColorHolder{ID: c.ID, Title: c.Title}
			if board.ColorName(c.Color) == "" {
				offPalette = append(offPalette, c.Color)
			}
		}
	}

	entries := make([]output.ColorEntry, 0, len(board.EpicPalette)+len(offPalette))
	for _, p := range board.EpicPalette {
		name := p.Name
		entries = append(entries, output.ColorEntry{
			Name:   &name,
			Hex:    p.Hex,
			HeldBy: holders[p.Hex],
		})
	}
	for _, hex := range offPalette {
		entries = append(entries, output.ColorEntry{
			Name:   nil,
			Hex:    hex,
			HeldBy: holders[hex],
		})
	}

	out := cmd.OutOrStdout()
	if asJSON {
		buf, err := output.Colors(output.ColorsEnvelope{Colors: entries})
		if err != nil {
			return err
		}
		_, err = out.Write(buf)
		return err
	}

	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		name := "-"
		if e.Name != nil {
			name = *e.Name
		}
		held := "free"
		if e.HeldBy != nil {
			held = e.HeldBy.ID + "  " + e.HeldBy.Title
		}
		rows = append(rows, []string{name, e.Hex, held})
	}
	_, err = out.Write([]byte(output.Table(rows, []string{"NAME", "HEX", "HELD BY"})))
	return err
}
