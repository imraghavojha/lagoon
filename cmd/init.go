package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/imraghavojha/lagoon/internal/config"
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

	// show preview so users can catch typos before the file is written
	fmt.Println()
	fmt.Printf("  packages:  %s\n", strings.Join(packages, ", "))
	fmt.Printf("  nixpkgs:   %s (pinned)\n", config.DefaultCommit[:8])
	fmt.Printf("  network:   %s\n", map[bool]string{true: "on", false: "off"}[network])
	fmt.Println()

	var confirm bool
	if err := huh.NewConfirm().
		Title("write lagoon.toml?").
		Affirmative("yes").
		Negative("no").
		Value(&confirm).
		Run(); err != nil {
		return err
	}
	if !confirm {
		fmt.Println("  not written.")
		return nil
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

	fmt.Println(ok("✓") + " created lagoon.toml")
	fmt.Println(warn("!") + " remember to commit it:  git add lagoon.toml && git commit")

	return nil
}
