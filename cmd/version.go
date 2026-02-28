package cmd

import (
	"fmt"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print the pinned nixpkgs commit",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("lagoon")
		fmt.Println("  nixpkgs commit: " + config.DefaultCommit)
	},
}
