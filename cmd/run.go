package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [command]",
	Short: "run a one-off command in the sandbox (like 'shell --cmd')",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdFlag = strings.Join(args, " ")
		return runShell(cmd, nil)
	},
}
