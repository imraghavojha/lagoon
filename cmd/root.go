package cmd

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// three styles, nothing more
var (
	ok   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render
	warn = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render
	fail = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render
)

var rootCmd = &cobra.Command{
	Use:   "lagoon",
	Short: "reproducible sandboxed shell environments using nix + bwrap",
	Long: `lagoon gives you a clean, isolated shell with exactly the tools you asked for.

no docker. no root. no daemons. one config file, one command.

run 'lagoon init' to set up a new environment.
run 'lagoon shell' to enter it.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(cleanCmd)
}
