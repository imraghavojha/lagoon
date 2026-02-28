package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "load a nix closure snapshot exported with 'lagoon export'",
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

func runImport(cmd *cobra.Command, args []string) error {
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
