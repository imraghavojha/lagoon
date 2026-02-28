package nix

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateGCRoots registers every store path in the environment as a nix GC root
// so nix-collect-garbage won't delete the sandbox packages.
// failures are silently ignored — the feature degrades gracefully if nix-store is unavailable.
func CreateGCRoots(cacheDir string, env *ResolvedEnv) {
	gcDir := filepath.Join(cacheDir, "gcroots")
	os.MkdirAll(gcDir, 0755)
	for _, entry := range strings.Split(env.PATH, ":") {
		storePath := filepath.Dir(entry)
		if !strings.HasPrefix(storePath, "/nix/store/") {
			continue
		}
		link := filepath.Join(gcDir, contentSum([]byte(storePath)))
		os.Remove(link) // remove stale link before recreating
		if err := os.Symlink(storePath, link); err == nil {
			exec.Command("nix-store", "--add-indirect-root", link).Run()
		}
	}
}
