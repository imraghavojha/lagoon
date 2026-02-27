package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/kuldeepojha/lagoon/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "create a lagoon.toml in the current directory",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	// overwrite guard — ask before clobbering an existing config
	if _, err := os.Stat(config.Filename); err == nil {
		var overwrite bool
		if err := huh.NewConfirm().
			Title("lagoon.toml already exists. overwrite?").
			Value(&overwrite).
			Run(); err != nil {
			return err
		}
		if !overwrite {
			fmt.Println("not overwriting. exiting.")
			return nil
		}
	}

	fmt.Println()
	fmt.Println("tip: search for package names at https://search.nixos.org/packages")
	fmt.Println()

	var rawPackages string
	var network bool

	// one form, two fields — packages and network toggle
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("packages (space-separated)").
				Placeholder("python311 ffmpeg cowsay").
				Value(&rawPackages).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return errors.New("at least one package is required")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("network access inside sandbox?").
				Affirmative("yes").
				Negative("no").
				Value(&network),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	packages := strings.Fields(rawPackages)
	profile := "minimal"
	if network {
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
