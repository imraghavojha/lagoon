package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
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

	envFile := filepath.Join(projectCacheDir(absPath), "env.json")

	fmt.Println("  packages: " + strings.Join(cfg.Packages, " "))
	fmt.Println("  profile:  " + cfg.Profile)

	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		fmt.Println(warn("!") + " not cached — run 'lagoon shell' to build")
	} else {
		fmt.Println(ok("✓") + " cached — next 'lagoon shell' starts instantly")
	}

	return nil
}
