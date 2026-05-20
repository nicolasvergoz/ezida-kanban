package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/board"
	"github.com/nicolasvergoz/ezida-kanban/internal/tty"
)

// rmFlags carries flags parsed by the `rm` command.
type rmFlags struct {
	yes bool
}

// rmIO bundles the readers/writers used by runRm. Tests inject pipes or
// buffers; production wires os.Stdin / os.Stderr. interactive reports
// whether the IO should be treated as a real terminal (drives the
// JSON-mode vs prompt branch).
type rmIO struct {
	in          io.Reader
	err         io.Writer
	interactive bool
}

// NewRmCmd builds the `ezida rm <id>` command.
func NewRmCmd(jsonOut *bool) *cobra.Command {
	f := rmFlags{}
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a card (with interactive confirmation by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			io := rmIO{
				in:          os.Stdin,
				err:         cmd.ErrOrStderr(),
				interactive: tty.IsTTY(os.Stdin) && tty.IsTTY(os.Stdout),
			}
			return runRm(cmd, BoardPath, args[0], f, *jsonOut, io)
		},
	}
	cmd.Flags().BoolVar(&f.yes, "yes", false, "skip interactive confirmation")
	return cmd
}

// runRm is the testable run body for `ezida rm`.
func runRm(cmd *cobra.Command, path, id string, f rmFlags, asJSON bool, rio rmIO) error {
	if asJSON && !f.yes {
		return &InteractiveRequiredError{Hint: "use --yes with --json"}
	}
	b, err := board.Load(path)
	if err != nil {
		return err
	}
	idx := indexCardByID(b.Cards, id)
	if idx < 0 {
		return &CardNotFoundError{ID: id}
	}
	title := b.Cards[idx].Title

	if !f.yes {
		if !rio.interactive {
			return &InteractiveRequiredError{Hint: "use --yes for non-interactive contexts"}
		}
		ok, err := promptConfirm(rio.err, rio.in, fmt.Sprintf(
			`Delete card %s %q? [y/N] `, id, title))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(rio.err, "aborted")
			return nil
		}
	}

	b.Cards = slices.Delete(b.Cards, idx, idx+1)
	if err := board.Save(path, b); err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if asJSON {
		_, err = fmt.Fprintf(out, "{\"id\":%q,\"deleted\":true}\n", id)
		return err
	}
	_, err = fmt.Fprintf(out, "removed %s\n", id)
	return err
}

// promptConfirm writes msg to w, reads one line from r, and returns
// true iff the trimmed lowercased response equals "y". An io.EOF on
// read is treated as a non-affirmative answer (returns false, nil).
func promptConfirm(w io.Writer, r io.Reader, msg string) (bool, error) {
	if _, err := fmt.Fprint(w, msg); err != nil {
		return false, err
	}
	reader := bufio.NewReader(r)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y", nil
}
