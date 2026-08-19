package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/tty"
)

// archiveColumnFlags carries flags parsed by `archive column`.
type archiveColumnFlags struct {
	yes bool
}

// NewArchiveCmd builds the `ezida archive <id>` command and its
// `column` / `list` / `get` subcommands. A bare id argument dispatches
// to runArchive; cobra resolves any first argument matching a
// subcommand name to that subcommand before the parent's RunE ever
// sees it. Card ids are six characters of [0-9a-z], so an id literally
// spelled "column", "list" or "get" would be shadowed — accepted as a
// documented limitation (see docs/usage.md Known limitations).
func NewArchiveCmd(jsonOut *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive [id]",
		Short: "Archive a card out of the active board, or manage the archive with a subcommand",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runArchive(cmd, BoardPath, board.ArchivePathFor(BoardPath), args[0], *jsonOut)
		},
	}
	cmd.AddCommand(newArchiveColumnCmd(jsonOut))
	cmd.AddCommand(newArchiveListCmd(jsonOut))
	cmd.AddCommand(newArchiveGetCmd(jsonOut))
	return cmd
}

func newArchiveColumnCmd(jsonOut *bool) *cobra.Command {
	f := archiveColumnFlags{}
	cmd := &cobra.Command{
		Use:   "column <name>",
		Short: "Archive every card in a column, leaving the column itself in place",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rio := rmIO{
				in:          os.Stdin,
				err:         cmd.ErrOrStderr(),
				interactive: tty.IsTTY(os.Stdin) && tty.IsTTY(os.Stdout),
			}
			return runArchiveColumn(cmd, BoardPath, board.ArchivePathFor(BoardPath), args[0], f, *jsonOut, rio)
		},
	}
	cmd.Flags().BoolVar(&f.yes, "yes", false, "skip interactive confirmation when the cascade leaves the column")
	return cmd
}

// runArchive is the testable run body for `ezida archive <id>`.
func runArchive(cmd *cobra.Command, boardPath, archivePath, id string, asJSON bool) error {
	at := nowFunc().UTC().Truncate(time.Second)
	var archived []board.ArchivedCard
	var staysWithEpic string

	err := mutateArchiveAndSave(boardPath, archivePath, func(b *board.Board, a *board.Archive) error {
		for _, c := range b.Cards {
			if c.ID == id {
				// ArchiveCard cascades only to id's children, never to
				// its own parent — so a non-empty Epic here always
				// names a card that stays on the board.
				staysWithEpic = c.Epic
				break
			}
		}
		var aerr error
		archived, aerr = board.ArchiveCard(b, a, id, at)
		if aerr != nil {
			return asBoardError(aerr)
		}
		return nil
	})
	if err != nil {
		return err
	}

	cascaded := make([]string, 0, len(archived))
	for _, c := range archived {
		if c.ID != id {
			cascaded = append(cascaded, c.ID)
		}
	}

	out := cmd.OutOrStdout()
	if asJSON {
		buf, merr := json.Marshal(struct {
			ID       string   `json:"id"`
			Archived bool     `json:"archived"`
			Cascaded []string `json:"cascaded"`
		}{ID: id, Archived: true, Cascaded: cascaded})
		if merr != nil {
			return merr
		}
		_, err = fmt.Fprintf(out, "%s\n", buf)
		return err
	}

	if _, err := fmt.Fprintln(out, id); err != nil {
		return err
	}
	if len(cascaded) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "also archived %d card(s) belonging to this epic: %s\n",
			len(cascaded), strings.Join(cascaded, ", "))
	}
	if staysWithEpic != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: this card belonged to epic %s, which stays on the board\n", staysWithEpic)
	}
	return nil
}

// runArchiveColumn is the testable run body for `ezida archive column`.
//
// The board and archive are loaded directly (not via
// mutateArchiveAndSave) because board.ArchiveColumn's result must be
// inspected — and possibly declined — before anything is persisted.
func runArchiveColumn(cmd *cobra.Command, boardPath, archivePath, name string, f archiveColumnFlags, asJSON bool, rio rmIO) error {
	b, err := board.Load(boardPath)
	if err != nil {
		return err
	}
	a, err := loadArchive(archivePath)
	if err != nil {
		return err
	}

	at := nowFunc().UTC().Truncate(time.Second)
	direct, cascaded, err := board.ArchiveColumn(b, a, name, at)
	if err != nil {
		return asBoardError(err)
	}

	if len(cascaded) > 0 {
		if !f.yes {
			if asJSON {
				return &InteractiveRequiredError{Hint: "use --yes with --json"}
			}
			if !rio.interactive {
				return &InteractiveRequiredError{Hint: "use --yes for non-interactive contexts"}
			}
			ok, perr := promptConfirm(rio.err, rio.in, fmt.Sprintf(
				"Archiving column %q also archives %d card(s) in other columns. Continue? [y/N] ",
				name, len(cascaded)))
			if perr != nil {
				return perr
			}
			if !ok {
				fmt.Fprintln(rio.err, "aborted")
				return nil
			}
		}
	}

	directIDs := archivedIDs(direct)
	cascadedIDs := archivedIDs(cascaded)

	if len(directIDs) > 0 || len(cascadedIDs) > 0 {
		if err := board.SaveArchive(archivePath, a); err != nil {
			return err
		}
		if err := board.Save(boardPath, b); err != nil {
			return err
		}
	}

	return printArchiveColumnResult(cmd, asJSON, name, directIDs, cascadedIDs)
}

func printArchiveColumnResult(cmd *cobra.Command, asJSON bool, name string, direct, cascaded []string) error {
	out := cmd.OutOrStdout()
	if asJSON {
		buf, merr := json.Marshal(struct {
			Column   string   `json:"column"`
			Archived []string `json:"archived"`
			Cascaded []string `json:"cascaded"`
		}{Column: name, Archived: direct, Cascaded: cascaded})
		if merr != nil {
			return merr
		}
		_, err := fmt.Fprintf(out, "%s\n", buf)
		return err
	}
	total := len(direct) + len(cascaded)
	if _, err := fmt.Fprintf(out, "archived %d cards from %q\n", total, name); err != nil {
		return err
	}
	if len(cascaded) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "also archived %d card(s) from other columns: %s\n",
			len(cascaded), strings.Join(cascaded, ", "))
	}
	return nil
}

// archivedIDs extracts ids from a slice of ArchivedCard, always
// returning a non-nil slice so JSON output emits [] rather than null.
func archivedIDs(cards []board.ArchivedCard) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.ID)
	}
	return out
}
