package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/engine"
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
	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml found — run 'lagoon init' first")
	}

	// macOS: portability means saving the container image as an OCI tar
	if engine.UseContainers() {
		if len(args) == 0 {
			return fmt.Errorf("on macOS, save needs a filename: lagoon save runtime.tar")
		}
		return saveImageArchive(cfg, args[0], "Lagoon save")
	}

	out, label, closeOut, err := saveOutput(args)
	if err != nil {
		return err
	}
	defer closeOut()

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

// saveImageArchive exports the project's container image as an OCI tar (macOS).
func saveImageArchive(cfg *config.Config, outFile, title string) error {
	eng, err := engine.Detect()
	if err != nil {
		return err
	}
	if err := eng.EnsureRunning(); err != nil {
		return err
	}
	image := engine.ImageFor(cfg)
	if !eng.HasImage(image) {
		pull := eng.Pull(image)
		pull.Stdout = os.Stderr
		pull.Stderr = os.Stderr
		if err := pull.Run(); err != nil {
			return fmt.Errorf("pulling %s: %w", image, err)
		}
	}
	fmt.Fprintln(os.Stderr, ui.Card(title,
		ui.Chip("engine", eng.Name(), ui.Good),
		ui.Chip("image", image, ui.Accent),
		ui.Chip("target", outFile, ui.Accent),
		ui.Bullet(ui.Dim.Render("•"), "OCI tar — load anywhere with lagoon load / docker load"),
	))
	save := eng.SaveImage(image, outFile)
	save.Stdout = os.Stderr
	save.Stderr = os.Stderr
	if err := save.Run(); err != nil {
		return err
	}
	if info, err := os.Stat(outFile); err == nil {
		fmt.Fprintln(os.Stderr, ui.OK.Render("✓ ")+"saved "+outFile+" ("+project.FormatBytes(info.Size())+")")
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
