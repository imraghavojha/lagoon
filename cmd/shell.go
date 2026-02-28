package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/imraghavojha/lagoon/internal/preflight"
	"github.com/imraghavojha/lagoon/internal/sandbox"
	"github.com/spf13/cobra"
)

var (
	cmdFlag  string
	envFlags []string
	memFlag  string
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "enter the sandboxed environment defined in lagoon.toml",
	RunE:  runShell,
}

func init() {
	shellCmd.Flags().StringVar(&cmdFlag, "cmd", "", "run a one-off command instead of an interactive shell")
	shellCmd.Flags().StringArrayVarP(&envFlags, "env", "e", nil, "set env var in sandbox (KEY=VALUE)")
	shellCmd.Flags().StringVarP(&memFlag, "memory", "m", "", "limit sandbox memory via systemd-run (e.g. 512m, 1g)")
}

func runShell(cmd *cobra.Command, args []string) error {
	// check that bwrap, nix-shell, and user namespaces are available
	if err := preflight.RunAll(); err != nil {
		fmt.Fprintln(os.Stderr, fail("✗")+" "+err.Error())
		os.Exit(1)
	}

	// load lagoon.toml — offer to run init inline if it's missing
	cfg, err := config.Read(config.Filename)
	if err != nil {
		var doInit bool
		if herr := huh.NewConfirm().
			Title("No lagoon.toml found. Run lagoon init now?").
			Affirmative("yes").
			Negative("no").
			Value(&doInit).
			Run(); herr != nil || !doInit {
			fmt.Fprintln(os.Stderr, fail("✗")+" no lagoon.toml. run 'lagoon init' first.")
			os.Exit(1)
		}
		if err := runInit(nil, nil); err != nil {
			return err
		}
		cfg, err = config.Read(config.Filename)
		if err != nil {
			return fmt.Errorf("reading config after init: %w", err)
		}
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

		// run nix-shell with a bubbletea spinner showing live progress
		progressCh := make(chan string, 50)
		resultCh := make(chan struct {
			env *nix.ResolvedEnv
			err error
		}, 1)
		go func() {
			env, err := nix.Resolve(shellNixPath, progressCh)
			close(progressCh)
			resultCh <- struct {
				env *nix.ResolvedEnv
				err error
			}{env, err}
		}()
		tea.NewProgram(newBuildModel(progressCh)).Run()
		r := <-resultCh
		if r.err != nil {
			fmt.Fprintln(os.Stderr, r.err.Error())
			os.Exit(1)
		}
		resolved = r.env
		_ = nix.SaveCache(cacheDir, resolved, sum)
		nix.CreateGCRoots(cacheDir, resolved)
	} else {
		fmt.Println(ok("✓") + " environment ready")
	}

	// banner so users know they're inside the sandbox
	netStr := "off"
	if cfg.Profile == "network" {
		netStr = "on"
	}
	memStr := ""
	if memFlag != "" {
		memStr = " │ mem: " + strings.ToUpper(memFlag)
	}
	fmt.Printf("\n%s │ %s │ /workspace │ network: %s%s\n",
		ok("lagoon"), strings.Join(cfg.Packages, "  "), netStr, memStr)
	fmt.Println("  type 'exit' to return to host shell")
	fmt.Println()

	// record pid so 'lagoon stats' can find this sandbox (same pid after syscall.Exec)
	writePIDFile(cacheDir, absPath, cfg.Packages)

	// replace this process with bwrap — no cleanup needed on exit
	return sandbox.Enter(cfg, resolved, absPath, cmdFlag, memFlag, envFlags)
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
