package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "verify lagoon.toml and validate the nix closure",
	Long: `lagoon check

Validates lagoon.toml structure and confirms every package exists in nixpkgs.`,
	RunE: runCheck,
}

func runCheck(cmd *cobra.Command, args []string) error {
	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml — run 'lagoon init' first")
	}

	// structural validation (was lint)
	var errs []string
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
	seen := map[string]bool{}
	for _, pkg := range cfg.Packages {
		if seen[pkg] {
			errs = append(errs, "duplicate package: "+pkg)
		}
		seen[pkg] = true
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Println(fail("✗") + "  " + e)
		}
		return errors.New("lagoon.toml is invalid")
	}

	// network package search (was lint)
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
		if slices.ContainsFunc(results, func(r nixPkg) bool { return r.name == pkg }) {
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
		return errors.New("check failed")
	}
	if !offline {
		fmt.Println(ok("✓") + " all packages found in nixpkgs")
	}
	return nil
}
