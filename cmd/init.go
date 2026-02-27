package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kuldeepojha/lagoon/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "create a lagoon.toml in the current directory",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	scanner := bufio.NewScanner(os.Stdin)

	// check if config already exists and ask before overwriting
	if _, err := os.Stat(config.Filename); err == nil {
		fmt.Printf("lagoon.toml already exists. overwrite? (y/N) ")
		scanner.Scan()
		if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
			fmt.Println("not overwriting. exiting.")
			return nil
		}
	}

	fmt.Println()
	fmt.Println("lagoon: no lagoon.toml found.")
	fmt.Println()
	fmt.Println("tip: search for package names at https://search.nixos.org/packages")
	fmt.Println()
	fmt.Printf("what packages do you need? (space-separated)\n> ")

	scanner.Scan()
	raw := strings.TrimSpace(scanner.Text())
	if raw == "" {
		return fmt.Errorf("no packages specified")
	}
	packages := strings.Fields(raw)

	fmt.Printf("\nuse network access inside sandbox? (y/N)\n> ")
	scanner.Scan()
	profile := "minimal"
	if strings.ToLower(strings.TrimSpace(scanner.Text())) == "y" {
		profile = "network"
	}

	cfg := &config.Config{
		Packages:      packages,
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       profile,
	}

	if err := config.Write(config.Filename, cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Println()
	fmt.Println(ok("✓") + " created lagoon.toml")
	fmt.Printf("  packages: %s\n", strings.Join(packages, ", "))
	fmt.Printf("  nixpkgs: pinned to %s (default)\n", config.DefaultCommit[:7])
	fmt.Printf("  profile: %s\n", profile)
	fmt.Println()
	fmt.Println(warn("!") + " remember to commit lagoon.toml to version control:")
	fmt.Println("    git add lagoon.toml && git commit -m \"add lagoon environment\"")

	return nil
}
