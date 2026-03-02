package nix

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateGCRoots registers the full transitive closure of all sandbox packages as nix GC roots
// so nix-collect-garbage won't delete shared libs or other transitive deps.
// failures are silently ignored — the feature degrades gracefully if nix-store is unavailable.
func CreateGCRoots(cacheDir string, env *ResolvedEnv) {
	gcDir := filepath.Join(cacheDir, "gcroots")
	os.MkdirAll(gcDir, 0755)

	// collect unique top-level store paths from PATH
	tops := map[string]bool{}
	for _, entry := range strings.Split(env.PATH, ":") {
		if p := filepath.Dir(entry); strings.HasPrefix(p, "/nix/store/") {
			tops[p] = true
		}
	}

	for top := range tops {
		// -qR prints the full transitive closure, one path per line
		out, err := exec.Command("nix-store", "-qR", top).Output()
		if err != nil {
			out = []byte(top) // fallback: protect at least the top-level path
		}
		for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if !strings.HasPrefix(dep, "/nix/store/") {
				continue
			}
			link := filepath.Join(gcDir, contentSum([]byte(dep)))
			os.Remove(link)
			if err := os.Symlink(dep, link); err == nil {
				exec.Command("nix-store", "--add-indirect-root", link).Run()
			}
		}
	}
}
