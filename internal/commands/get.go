package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// NewGetCmd builds the `ezida get <id>` command.
func NewGetCmd(jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Show the full detail of a single card",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGet(cmd, BoardPath, args[0], *jsonOut)
		},
	}
	return cmd
}

// runGet is the testable run body for `ezida get`.
func runGet(cmd *cobra.Command, path, id string, asJSON bool) error {
	b, err := board.Load(path)
	if err != nil {
		return err
	}
	var found *board.Card
	for i := range b.Cards {
		if b.Cards[i].ID == id {
			found = &b.Cards[i]
			break
		}
	}
	if found == nil {
		return &CardNotFoundError{ID: id}
	}

	// A card is either a child or a parent, never both — one-level
	// nesting makes that state unrepresentable — so at most one of the
	// two blocks below fills in.
	parent := board.ParentOf(b, found.ID)
	children := board.ChildrenOf(b, found.ID)

	out := cmd.OutOrStdout()
	if asJSON {
		tags := found.Tags
		if tags == nil {
			tags = []string{}
		}
		card := output.GetCard{
			ID:          found.ID,
			Title:       found.Title,
			Column:      found.Column,
			Priority:    found.Priority,
			Tags:        tags,
			Description: found.Description,
			Color:       found.Color,
			CreatedAt:   found.CreatedAt,
			UpdatedAt:   found.UpdatedAt,
		}
		if parent != nil {
			card.Epic = &output.EpicRef{ID: parent.ID, Title: parent.Title}
		}
		if len(children) > 0 {
			refs := make([]output.ChildRef, 0, len(children))
			for _, c := range children {
				refs = append(refs, output.ChildRef{ID: c.ID, Title: c.Title, Column: c.Column})
			}
			done, total := board.EpicProgress(b, found.ID)
			card.Children = refs
			card.Progress = &output.Progress{Done: done, Total: total}
		}
		env := output.GetEnvelope{Card: card}
		buf, err := output.Get(env)
		if err != nil {
			return err
		}
		_, err = out.Write(buf)
		return err
	}

	pri := found.Priority
	if pri == "" {
		pri = "-"
	}
	tagsStr := "-"
	if len(found.Tags) > 0 {
		tagsStr = strings.Join(found.Tags, ", ")
	}
	kvs := []output.KV{
		{Key: "ID", Value: found.ID},
		{Key: "Title", Value: found.Title},
		{Key: "Column", Value: found.Column},
		{Key: "Priority", Value: pri},
		{Key: "Tags", Value: tagsStr},
	}
	if parent != nil {
		kvs = append(kvs, output.KV{
			Key:   "Epic",
			Value: fmt.Sprintf("%s  %s", parent.ID, parent.Title),
		})
	}
	if found.Color != "" {
		color := found.Color
		if name := board.ColorName(color); name != "" {
			color = fmt.Sprintf("%s (%s)", color, name)
		}
		kvs = append(kvs, output.KV{Key: "Color", Value: color})
	}
	if len(children) > 0 {
		done, total := board.EpicProgress(b, found.ID)
		kvs = append(kvs, output.KV{
			Key:   "Progress",
			Value: fmt.Sprintf("%d/%d", done, total),
		})
	}
	kvs = append(kvs,
		output.KV{Key: "Created", Value: found.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")},
		output.KV{Key: "Updated", Value: found.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")},
	)
	_, _ = out.Write([]byte(output.KeyValue(kvs)))
	if len(children) > 0 {
		_, _ = out.Write([]byte("\nChildren:\n"))
		for _, c := range children {
			marker := " "
			if b.Board.IsDoneColumn(c.Column) {
				marker = doneMarker
			}
			fmt.Fprintf(out, "  %s %s  %-10s %s\n", marker, c.ID, c.Column, c.Title)
		}
	}
	if found.Description != "" {
		_, _ = out.Write([]byte("\nDescription:\n"))
		desc := found.Description
		if !strings.HasSuffix(desc, "\n") {
			desc += "\n"
		}
		_, _ = out.Write([]byte(desc))
	}
	return nil
}
