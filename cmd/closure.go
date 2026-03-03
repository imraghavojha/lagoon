package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/imraghavojha/lagoon/internal/nix"
)

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

// envStorePaths extracts the top-level nix store paths from the environment PATH.
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
