package cmd

import (
	"fmt"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X github.com/imraghavojha/lagoon/cmd.version=..."
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version and pinned nixpkgs commit",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("lagoon " + version)
		fmt.Println("  nixpkgs commit: " + config.DefaultCommit)
	},
}
