package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
	Use:   "load <file>",
	Short: "import an environment from a .nar file",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoad,
}

func runLoad(cmd *cobra.Command, args []string) error {
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(os.Stderr, ok("→")+" importing from "+args[0]+"…")

	imp := exec.Command("nix-store", "--import")
	imp.Stdin = f
	imp.Stdout = os.Stdout
	imp.Stderr = os.Stderr
	return imp.Run()
}
