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
	Short: "beautiful Linux dev environments and tiny runtimes without Docker overhead",
	Long: `Lagoon is a Linux CLI for reproducible dev shells, small local service
stacks, offline portable runtimes, and Docker export when you need it.

Core flow:
  lagoon init             hardware-aware wizard and config preview
  lagoon shell            enter the reproducible dev workspace
  lagoon up               run services with a live dashboard
  lagoon ps               show machine/cache/process status
  lagoon save runtime.nar save for offline/lab/field machines
  lagoon load runtime.nar load an offline runtime
  lagoon docker image.tar export to Docker ecosystems`,
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
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(psCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(saveCmd)
	rootCmd.AddCommand(loadCmd)
	rootCmd.AddCommand(dockerCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(versionCmd)
}
