package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "show whether the current project's environment is cached",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Read(config.Filename)
	if err != nil {
		fmt.Println(warn("!") + " no lagoon.toml found — run 'lagoon init' first")
		return nil
	}

	absPath, err := filepath.Abs(".")
	if err != nil {
		return err
	}

	cacheDir := projectCacheDir(absPath)
	shellNixPath := filepath.Join(cacheDir, "shell.nix")

	fmt.Println("  packages: " + strings.Join(cfg.Packages, " "))
	fmt.Println("  profile:  " + cfg.Profile)

	// generate shell.nix to compute the current sum (no-op if content unchanged)
	sum, _ := nix.GenerateShellNix(cfg, shellNixPath)
	if _, hit := nix.LoadCache(cacheDir, sum); hit {
		fmt.Println(ok("✓") + " cached — next 'lagoon shell' starts instantly")
	} else {
		fmt.Println(warn("!") + " not cached — run 'lagoon shell' to build")
	}

	return nil
}
