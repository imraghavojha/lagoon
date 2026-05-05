package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/imraghavojha/lagoon/internal/project"
	"github.com/imraghavojha/lagoon/internal/ui"
	"github.com/spf13/cobra"
)

var saveCmd = &cobra.Command{
	Use:   "save [file]",
	Short: "save the environment for offline portability",
	Long: `lagoon save runtime.nar
lagoon save > runtime.nar

Snapshots every nix store path the environment needs. The resulting file can
be transferred to an air-gapped machine and loaded with 'lagoon load'. Uses
'nix-store --export' under the hood — no registry, no internet required after
export.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSave,
}

func runSave(cmd *cobra.Command, args []string) error {
	out, label, closeOut, err := saveOutput(args)
	if err != nil {
		return err
	}
	defer closeOut()

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

	fmt.Fprintln(os.Stderr, ui.Card("Lagoon save",
		ui.Chip("target", label, ui.Accent),
		ui.Chip("store paths", fmt.Sprint(len(paths)), ui.Good),
		ui.Chip("cache", "warm", ui.Good),
		ui.Bullet(ui.Dim.Render("•"), "portable NAR can be loaded offline with lagoon load"),
	))

	exp := exec.Command("nix-store", append([]string{"--export"}, paths...)...)
	exp.Stdout = out
	exp.Stderr = os.Stderr
	if err := exp.Run(); err != nil {
		return err
	}
	if file, ok := out.(*os.File); ok && file != os.Stdout {
		if info, err := file.Stat(); err == nil {
			fmt.Fprintln(os.Stderr, ui.OK.Render("✓ ")+"saved "+label+" ("+project.FormatBytes(info.Size())+")")
		}
	}
	return nil
}

func saveOutput(args []string) (io.Writer, string, func(), error) {
	if len(args) == 0 {
		if info, err := os.Stdout.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return nil, "", func() {}, fmt.Errorf("stdout is a terminal — use: lagoon save runtime.nar")
		}
		return os.Stdout, "stdout", func() {}, nil
	}
	f, err := os.Create(args[0])
	if err != nil {
		return nil, "", func() {}, err
	}
	return f, args[0], func() { _ = f.Close() }, nil
}
