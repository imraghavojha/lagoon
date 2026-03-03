package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch <command>",
	Short: "run a command in the sandbox, restart it when files change",
	Long: `lagoon watch "python3 server.py"

Starts the command inside the sandbox and watches the project directory for
file changes, restarting automatically when anything changes.
Requires watchexec on PATH — add it to your lagoon.toml packages.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runWatch,
}

func runWatch(cmd *cobra.Command, args []string) error {
	watchexec, err := exec.LookPath("watchexec")
	if err != nil {
		return fmt.Errorf("watchexec not found — add it to packages in lagoon.toml")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	argv := append([]string{"watchexec", "--", self, "run"}, args...)
	return syscall.Exec(watchexec, argv, os.Environ())
}
