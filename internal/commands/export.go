package commands

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// NewExportCmd builds the `ezida export` command. The command emits a
// single JSON envelope to stdout that mirrors the viewer's GET
// /api/board response shape so a static snapshot of the board can feed
// the demo viewer in site/demo/.
func NewExportCmd(jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the full board as a JSON snapshot (mirrors GET /api/board)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExport(cmd, BoardPath, *jsonOut)
		},
	}
	return cmd
}

// runExport loads the board and writes the ExportEnvelope to stdout.
// asJSON is currently the only supported output mode; passing false
// still emits JSON (the command is JSON-only by purpose) — the flag is
// honoured so error envelopes follow the same JSON-on-error rules as
// the rest of the CLI.
func runExport(cmd *cobra.Command, path string, asJSON bool) error {
	_ = asJSON
	b, err := board.Load(path)
	if err != nil {
		return err
	}

	counts := make(map[string]int, len(b.Board.Columns))
	for _, col := range b.Board.Columns {
		counts[col] = 0
	}
	for _, c := range b.Cards {
		counts[c.Column]++
	}

	cards := make([]output.ExportCard, 0, len(b.Cards))
	for _, c := range b.Cards {
		tags := c.Tags
		if tags == nil {
			tags = []string{}
		}
		cards = append(cards, output.ExportCard{
			ID:          c.ID,
			Title:       c.Title,
			Column:      c.Column,
			Priority:    c.Priority,
			Tags:        tags,
			Description: c.Description,
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.UpdatedAt,
		})
	}

	env := output.ExportEnvelope{
		SchemaVersion:  b.SchemaVersion,
		Columns:        b.Board.Columns,
		Priorities:     b.Board.Priorities,
		CardsPerColumn: counts,
		Cards:          cards,
		ProjectName:    resolveProjectName(path),
	}
	buf, err := output.Export(env)
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(buf)
	return err
}

// resolveProjectName mirrors internal/server.resolveProjectName: the
// parent-directory name of the resolved board path, with fallback to
// "Ezida" when the basename is empty, ".", or the platform separator.
// Duplicated rather than imported to avoid a commands → server
// dependency edge; the logic is 5 lines and stable.
func resolveProjectName(boardPath string) string {
	abs, err := filepath.Abs(boardPath)
	if err != nil {
		abs = boardPath
	}
	name := filepath.Base(filepath.Dir(abs))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "Ezida"
	}
	return name
}
