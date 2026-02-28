package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/imraghavojha/lagoon/internal/preflight"
	"github.com/imraghavojha/lagoon/internal/sandbox"
	"github.com/spf13/cobra"
)

var (
	cmdFlag  string
	envFlags []string
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "enter the sandboxed environment defined in lagoon.toml",
	RunE:  runShell,
}

func init() {
	shellCmd.Flags().StringVar(&cmdFlag, "cmd", "", "run a one-off command instead of an interactive shell")
	shellCmd.Flags().StringArrayVarP(&envFlags, "env", "e", nil, "set env var in sandbox (KEY=VALUE)")
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

	// figure out where to put the generated shell.nix
	absPath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}
	cacheDir := projectCacheDir(absPath)
	shellNixPath := filepath.Join(cacheDir, "shell.nix")

	// write shell.nix (skips write if content unchanged)
	sum, err := nix.GenerateShellNix(cfg, shellNixPath)
	if err != nil {
		return fmt.Errorf("generating shell.nix: %w", err)
	}

	// warm start: skip nix-shell entirely if we have a matching cached env
	resolved, hit := nix.LoadCache(cacheDir, sum)
	if !hit {
		// arm warning only matters on cold starts — warm starts are instant
		if runtime.GOARCH == "arm64" {
			fmt.Println(warn("!") + " arm: first run may take 10-60 min to compile packages")
			fmt.Println("  this only happens once. subsequent runs start in under a second.")
		}
		fmt.Println(warn("!") + " building environment...")
		resolved, err = nix.Resolve(shellNixPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		_ = nix.SaveCache(cacheDir, resolved, sum)
	} else {
		fmt.Println(ok("✓") + " environment ready")
	}

	// print banner so users know they're inside the sandbox
	netStr := "off"
	if cfg.Profile == "network" {
		netStr = "on"
	}
	fmt.Printf("\n%s │ %s │ /workspace │ network: %s\n",
		ok("lagoon"), strings.Join(cfg.Packages, "  "), netStr)
	fmt.Println("  type 'exit' to return to host shell\n")

	// replace this process with bwrap — no cleanup needed on exit
	return sandbox.Enter(cfg, resolved, absPath, cmdFlag, envFlags)
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
