package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/spf13/cobra"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "check lagoon.toml is valid and packages exist in nixpkgs",
	RunE:  runLint,
}

func runLint(cmd *cobra.Command, args []string) error {
	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml — run 'lagoon init' first")
	}

	var errs []string

	// structural validation
	if len(cfg.Packages) == 0 {
		errs = append(errs, "packages list is empty")
	}
	if cfg.NixpkgsCommit == "" {
		errs = append(errs, "nixpkgs_commit is missing")
	}
	if cfg.NixpkgsSHA256 == "" {
		errs = append(errs, "nixpkgs_sha256 is missing")
	}
	if cfg.Profile != "minimal" && cfg.Profile != "network" {
		errs = append(errs, `profile must be "minimal" or "network"`)
	}

	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Println(fail("✗") + "  " + e)
		}
		return errors.New("lagoon.toml is invalid")
	}

	fmt.Println("  checking " + fmt.Sprint(len(cfg.Packages)) + " packages against nixpkgs…")
	fmt.Println()

	var bad []string
	offline := false

	for _, pkg := range cfg.Packages {
		results, err := queryNixpkgs(pkg)
		if err != nil {
			offline = true
			fmt.Println(warn("?") + "  " + pkg + "  (offline — couldn't verify)")
			continue
		}
		found := false
		for _, r := range results {
			if r.name == pkg {
				found = true
				break
			}
		}
		if found {
			fmt.Println(ok("✓") + "  " + pkg)
		} else {
			bad = append(bad, pkg)
			fmt.Println(fail("✗") + "  " + pkg)
		}
	}

	fmt.Println()
	if offline {
		fmt.Println(warn("!") + " some packages couldn't be verified (offline or API unavailable)")
	}
	if len(bad) > 0 {
		fmt.Printf("%s %d package(s) not found: %s\n", fail("✗"), len(bad), strings.Join(bad, ", "))
		fmt.Println("  search at: https://search.nixos.org/packages")
		return errors.New("lint failed")
	}
	if !offline {
		fmt.Println(ok("✓") + " all packages found in nixpkgs")
	}
	return nil
}
