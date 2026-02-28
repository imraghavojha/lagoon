package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "stream the nix closure to stdout — pipe to a file for offline use",
	Long: `lagoon export > myenv.nar

Snapshots every nix store path the environment needs. The resulting file can
be transferred to an air-gapped machine and loaded with 'lagoon import'.
Uses 'nix-store --export' under the hood — no registry, no internet required
after export.`,
	RunE: runExport,
}

func runExport(cmd *cobra.Command, args []string) error {
	// refuse to dump binary NAR data to a terminal — caller must redirect
	if info, err := os.Stdout.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return fmt.Errorf("stdout is a terminal — redirect to a file: lagoon export > myenv.nar")
	}

	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml found — run 'lagoon init' first")
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
		return fmt.Errorf("nix-store -qR: %w", err)
	}

	fmt.Fprintln(os.Stderr, ok("→")+" exporting "+fmt.Sprint(len(paths))+" store paths…")

	// stream the export to stdout so the caller can pipe or redirect it
	exp := exec.Command("nix-store", append([]string{"--export"}, paths...)...)
	exp.Stdout = os.Stdout
	exp.Stderr = os.Stderr
	return exp.Run()
}
