package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/nicolasvergoz/ezida-kanban/internal/commands"
	"github.com/nicolasvergoz/ezida-kanban/internal/output"
)

// version is overridable at build time via:
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/ezida
var version = "dev"

var (
	jsonOut bool
	noColor bool
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "ezida",
		Short:   "File-based Kanban for software projects",
		Long:    "ezida is a file-based Kanban CLI for software projects, backed by a single kanban.toml.",
		Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
	rootCmd.SetVersionTemplate("ezida version {{.Version}}\n")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "emit JSON to stdout")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.AddCommand(commands.NewInitCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewBoardCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewListCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewGetCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewAddCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewMoveCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewRmCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewEditCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewColumnsCmd(&jsonOut))
	rootCmd.AddCommand(commands.NewPrioritiesCmd(&jsonOut))
	// Silence cobra's default error rendering; output.Fail owns it.
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	return rootCmd
}

func main() {
	rootCmd := newRootCmd()
	// Pre-parse persistent flags to resolve color before any command runs.
	_ = rootCmd.ParseFlags(os.Args[1:])
	output.ConfigureColor(noColor)
	if err := rootCmd.Execute(); err != nil {
		output.Fail(err, jsonOut)
	}
}
