package commands

import (
	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/server"
)

// NewServeCmd builds the `ezida serve` command. jsonOut points at the
// root command's --json flag so future error envelopes (e.g.
// PORT_UNAVAILABLE surfaced by server.Run) flow through output.Fail
// with the right format.
func NewServeCmd(jsonOut *bool) *cobra.Command {
	var (
		port   int
		noOpen bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the viewer HTTP server on 127.0.0.1",
		Long: "Launch the Ezida viewer: an HTTP server on " +
			"127.0.0.1 that renders kanban.toml as a Kanban board. " +
			"Defaults to port 7777 with automatic fallback to the " +
			"next 10 ports if busy. Opens the user's default " +
			"browser unless --no-open is set. Blocks until SIGINT " +
			"or SIGTERM, then drains in-flight requests within 5s.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Run(server.Options{
				Port:   port,
				NoOpen: noOpen,
				Board:  BoardPath,
			})
		},
	}
	cmd.Flags().IntVar(&port, "port", 7777,
		"starting HTTP port (auto-fallback covers next 10)")
	cmd.Flags().BoolVar(&noOpen, "no-open", false,
		"skip launching the default browser on startup")
	return cmd
}
