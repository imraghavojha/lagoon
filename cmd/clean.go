package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "remove the cached shell.nix for the current project",
	RunE:  runClean,
}

func runClean(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	dir := projectCacheDir(absPath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Println("no cache found for this project.")
		return nil
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing cache: %w", err)
	}

	fmt.Println(ok("✓") + " removed cache: " + dir)
	return nil
}
