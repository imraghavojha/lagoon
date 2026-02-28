package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "verify the nix closure hasn't changed since baseline was set",
	Long: `lagoon verify computes a fingerprint of every store path in the environment
closure and compares it to a stored baseline. the baseline is created on
first run. if the closure changes (e.g. someone swapped packages via nix),
verify catches it.

unlike docker, nix paths are content-addressed — the fingerprint is a
cryptographic proof of exactly which binaries are in the environment.`,
	RunE: runVerify,
}

func runVerify(cmd *cobra.Command, args []string) error {
	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml — run 'lagoon init' first")
	}

	absPath, _ := filepath.Abs(".")
	cacheDir := projectCacheDir(absPath)
	shellNixPath := filepath.Join(cacheDir, "shell.nix")

	sum, err := nix.GenerateShellNix(cfg, shellNixPath)
	if err != nil {
		return err
	}

	resolved, hit := nix.LoadCache(cacheDir, sum)
	if !hit {
		return fmt.Errorf("no cached environment — run 'lagoon shell' first to build it")
	}

	paths, err := closurePaths(resolved)
	if err != nil {
		return fmt.Errorf("getting closure: %w", err)
	}

	current := closureFingerprint(paths)
	baselinePath := filepath.Join(cacheDir, "closure.fingerprint")

	stored, err := os.ReadFile(baselinePath)
	if err != nil {
		// first run — establish baseline
		if err := os.WriteFile(baselinePath, []byte(current), 0644); err != nil {
			return fmt.Errorf("saving baseline: %w", err)
		}
		fmt.Printf("%s baseline set — %d paths, fingerprint: %s…\n",
			ok("✓"), len(paths), current[:16])
		fmt.Println("  run 'lagoon verify' again to check against this baseline")
		return nil
	}

	if strings.TrimSpace(string(stored)) == current {
		fmt.Printf("%s verified — %d paths, fingerprint: %s… matches baseline\n",
			ok("✓"), len(paths), current[:16])
		return nil
	}

	fmt.Println(fail("✗") + " environment has changed since baseline was set")
	fmt.Printf("  baseline:  %s…\n", strings.TrimSpace(string(stored))[:16])
	fmt.Printf("  current:   %s…\n", current[:16])
	fmt.Println("  if this is expected, delete the baseline: rm " + baselinePath)
	return fmt.Errorf("verification failed")
}

// envStorePaths extracts the top-level nix store paths from the environment PATH.
// each PATH entry is a <storepath>/bin dir — taking its parent gives the store path.
func envStorePaths(env *nix.ResolvedEnv) []string {
	seen := map[string]bool{}
	var paths []string
	for _, entry := range strings.Split(env.PATH, ":") {
		sp := filepath.Dir(entry)
		if strings.HasPrefix(sp, "/nix/store/") && !seen[sp] {
			seen[sp] = true
			paths = append(paths, sp)
		}
	}
	return paths
}

// closurePaths returns the full transitive nix closure for all packages in the environment.
func closurePaths(resolved *nix.ResolvedEnv) ([]string, error) {
	roots := envStorePaths(resolved)
	if len(roots) == 0 {
		return nil, fmt.Errorf("no nix store paths in environment PATH")
	}
	out, err := exec.Command("nix-store", append([]string{"-qR"}, roots...)...).Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// closureFingerprint returns a hex sha256 of the sorted path list.
func closureFingerprint(paths []string) string {
	sorted := append([]string{}, paths...)
	sort.Strings(sorted)
	h := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return fmt.Sprintf("%x", h[:])
}
