package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
)

// unarchiveFlags carries flags parsed by `ezida unarchive`.
type unarchiveFlags struct {
	column string
}

// NewUnarchiveCmd builds the `ezida unarchive <id>` command.
func NewUnarchiveCmd(jsonOut *bool) *cobra.Command {
	f := unarchiveFlags{}
	cmd := &cobra.Command{
		Use:   "unarchive <id>",
		Short: "Restore an archived card (and its archived children) to the board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnarchive(cmd, BoardPath, board.ArchivePathFor(BoardPath), args[0], f, *jsonOut)
		},
	}
	cmd.Flags().StringVar(&f.column, "column", "", "restore into this column instead of the stored one")
	return cmd
}

// runUnarchive is the testable run body for `ezida unarchive`.
func runUnarchive(cmd *cobra.Command, boardPath, archivePath, id string, f unarchiveFlags, asJSON bool) error {
	var restored []board.Card
	var orphaned []string
	var relocated bool
	var originalColumn string

	err := mutateUnarchiveAndSave(boardPath, archivePath, func(b *board.Board, a *board.Archive) error {
		for _, c := range a.Cards {
			if c.ID == id {
				originalColumn = c.Column
				break
			}
		}
		var uerr error
		restored, orphaned, relocated, uerr = board.UnarchiveCard(b, a, id, f.column)
		if uerr != nil {
			return asBoardError(uerr)
		}
		return nil
	})
	if err != nil {
		return err
	}

	cascaded := make([]string, 0, len(restored))
	var destColumn string
	for _, c := range restored {
		if c.ID == id {
			destColumn = c.Column
			continue
		}
		cascaded = append(cascaded, c.ID)
	}
	if orphaned == nil {
		orphaned = []string{}
	}

	out := cmd.OutOrStdout()
	if asJSON {
		buf, merr := json.Marshal(struct {
			ID         string   `json:"id"`
			Unarchived bool     `json:"unarchived"`
			Cascaded   []string `json:"cascaded"`
			Orphaned   []string `json:"orphaned"`
			Column     string   `json:"column"`
			Relocated  bool     `json:"relocated"`
		}{ID: id, Unarchived: true, Cascaded: cascaded, Orphaned: orphaned, Column: destColumn, Relocated: relocated})
		if merr != nil {
			return merr
		}
		_, err = fmt.Fprintf(out, "%s\n", buf)
		return err
	}

	if _, err := fmt.Fprintln(out, id); err != nil {
		return err
	}
	errOut := cmd.ErrOrStderr()
	if relocated {
		fmt.Fprintf(errOut, "column %q no longer exists; restored into %q\n", originalColumn, destColumn)
	}
	if len(cascaded) > 0 {
		fmt.Fprintf(errOut, "also restored %d card(s) belonging to this epic: %s\n",
			len(cascaded), strings.Join(cascaded, ", "))
	}
	if len(orphaned) > 0 {
		fmt.Fprintf(errOut, "cleared epic on %d card(s) whose parent is not on the board: %s\n",
			len(orphaned), strings.Join(orphaned, ", "))
	}
	return nil
}
