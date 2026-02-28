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
		cmdFlag = shellQuoteArgs(args)
		return runShell(cmd, nil)
	},
}

// shellQuoteArgs joins args into a bash -c safe string — each arg is single-quoted.
// single quotes inside args are handled via the '\'' technique.
func shellQuoteArgs(args []string) string {
	escaped := make([]string, len(args))
	for i, a := range args {
		escaped[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(escaped, " ")
}

func init() {
	runCmd.Flags().StringVarP(&memFlag, "memory", "m", "", "limit sandbox memory via systemd-run (e.g. 512m, 1g)")
}
