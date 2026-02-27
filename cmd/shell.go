package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/imraghavojha/lagoon/internal/preflight"
	"github.com/imraghavojha/lagoon/internal/sandbox"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "enter the sandboxed environment defined in lagoon.toml",
	RunE:  runShell,
}

func runShell(cmd *cobra.Command, args []string) error {
	// check that bwrap, nix-shell, and user namespaces are available
	if err := preflight.RunAll(); err != nil {
		fmt.Fprintln(os.Stderr, fail("✗")+" "+err.Error())
		os.Exit(1)
	}

	// load lagoon.toml from current directory
	cfg, err := config.Read(config.Filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, fail("✗")+" no lagoon.toml found. run 'lagoon init' first.")
		os.Exit(1)
	}

	// warn early on arm — builds can take a very long time
	if runtime.GOARCH == "arm64" {
		fmt.Println(warn("!") + " arm detected: first run may take 10-60 minutes while packages compile.")
		fmt.Println("  this only happens once. subsequent runs use the cache.")
		fmt.Println("  do not interrupt this process.")
	}

	// figure out where to put the generated shell.nix
	absPath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	cacheDir := projectCacheDir(absPath)
	shellNixPath := filepath.Join(cacheDir, "shell.nix")

	// write the shell.nix from the config
	if err := nix.GenerateShellNix(cfg, shellNixPath); err != nil {
		return fmt.Errorf("generating shell.nix: %w", err)
	}

	fmt.Println("  building environment... (first run may take several minutes)")

	// run nix-shell to get the resolved paths we need for bwrap
	resolved, err := nix.Resolve(shellNixPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// replace this process with bwrap — no cleanup needed on exit
	return sandbox.Enter(cfg, resolved, absPath)
}

// projectCacheDir returns ~/.cache/lagoon/<8-char hash of project path>
func projectCacheDir(absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	id := fmt.Sprintf("%x", h[:4])
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "lagoon", id)
}
