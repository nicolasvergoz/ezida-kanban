package commands

import (
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

	out := cmd.OutOrStdout()
	if asJSON {
		tags := found.Tags
		if tags == nil {
			tags = []string{}
		}
		env := output.GetEnvelope{Card: output.GetCard{
			ID:          found.ID,
			Title:       found.Title,
			Column:      found.Column,
			Priority:    found.Priority,
			Tags:        tags,
			Description: found.Description,
			CreatedAt:   found.CreatedAt,
			UpdatedAt:   found.UpdatedAt,
		}}
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
		{Key: "Created", Value: found.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")},
		{Key: "Updated", Value: found.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z")},
	}
	_, _ = out.Write([]byte(output.KeyValue(kvs)))
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
