package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/spf13/cobra"
)

var checkReset bool

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "verify lagoon.toml and validate the nix closure",
	Long: `lagoon check

Validates lagoon.toml structure and confirms every package exists in nixpkgs.

If the environment has been built (lagoon shell), also verifies the nix closure
fingerprint against a stored baseline — catching unexpected package swaps.
The baseline is set automatically on first run.

Use --reset to wipe the baseline and re-establish it from the current closure.`,
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().BoolVar(&checkReset, "reset", false, "wipe and re-establish the closure baseline")
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
		return errors.New("check failed")
	}
	if !offline {
		fmt.Println(ok("✓") + " all packages found in nixpkgs")
	}

	// closure fingerprint (was verify) — only if environment is cached
	absPath, _ := filepath.Abs(".")
	cacheDir := projectCacheDir(absPath)
	shellNixPath := filepath.Join(cacheDir, "shell.nix")

	sum, err := nix.GenerateShellNix(cfg, shellNixPath)
	if err != nil {
		return err
	}
	resolved, hit := nix.LoadCache(cacheDir, sum)
	if !hit {
		fmt.Println(warn("!") + " environment not built yet — run 'lagoon shell' to enable closure verification")
		return nil
	}

	paths, err := closurePaths(resolved)
	if err != nil {
		return fmt.Errorf("getting closure: %w", err)
	}

	current := closureFingerprint(paths)
	baselinePath := filepath.Join(cacheDir, "closure.fingerprint")

	if checkReset {
		if err := os.WriteFile(baselinePath, []byte(current), 0644); err != nil {
			return fmt.Errorf("saving baseline: %w", err)
		}
		fmt.Printf("%s baseline reset — %d paths, fingerprint: %s…\n", ok("✓"), len(paths), current[:16])
		return nil
	}

	stored, err := os.ReadFile(baselinePath)
	if err != nil {
		// first run — set baseline automatically
		if err := os.WriteFile(baselinePath, []byte(current), 0644); err != nil {
			return fmt.Errorf("saving baseline: %w", err)
		}
		fmt.Printf("%s baseline set — %d paths, fingerprint: %s…\n", ok("✓"), len(paths), current[:16])
		fmt.Println("  run 'lagoon check' again to verify against this baseline")
		return nil
	}

	if strings.TrimSpace(string(stored)) == current {
		fmt.Printf("%s verified — %d paths, fingerprint: %s… matches baseline\n", ok("✓"), len(paths), current[:16])
		return nil
	}

	fmt.Println(fail("✗") + " environment has changed since baseline was set")
	fmt.Printf("  baseline:  %s…\n", strings.TrimSpace(string(stored))[:16])
	fmt.Printf("  current:   %s…\n", current[:16])
	fmt.Println("  run 'lagoon check --reset' to update the baseline")
	return fmt.Errorf("verification failed")
}
