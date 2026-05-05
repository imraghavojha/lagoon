package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/imraghavojha/lagoon/internal/project"
	"github.com/imraghavojha/lagoon/internal/ui"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker [file]",
	Short: "export the environment as a Docker image tar",
	Long: `lagoon docker image.tar
lagoon docker > image.tar

Builds a layered Docker image from the nix environment defined in lagoon.toml.
The resulting tar can be loaded with: docker load < image.tar. Use Lagoon
locally; export Docker when you need to cross into Docker ecosystems.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDocker,
}

func runDocker(cmd *cobra.Command, args []string) error {
	out, label, closeOut, err := dockerOutput(args)
	if err != nil {
		return err
	}
	defer closeOut()

	cfg, err := config.Read(config.Filename)
	if err != nil {
		return fmt.Errorf("no lagoon.toml found — run 'lagoon init' first")
	}

	absPath, _ := filepath.Abs(".")
	name := "lagoon-" + strings.ToLower(filepath.Base(absPath))
	cacheDir := projectCacheDir(absPath)
	dockerNixPath := filepath.Join(cacheDir, "docker.nix")

	if err := nix.GenerateDockerNix(cfg, dockerNixPath, name); err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, ui.Card("Lagoon docker",
		ui.Chip("image", name+":latest", ui.Accent),
		ui.Chip("target", label, ui.Good),
		ui.Bullet(ui.Dim.Render("•"), "use Lagoon locally; export Docker when needed"),
		ui.Bullet(ui.Dim.Render("•"), "build uses nixpkgs.dockerTools; Docker daemon not required"),
	))

	build := exec.Command("nix-build", "--no-out-link", dockerNixPath)
	build.Stderr = os.Stderr
	outBytes, err := build.Output()
	if err != nil {
		return fmt.Errorf("nix-build failed: %w", err)
	}

	imagePath := strings.TrimSpace(string(outBytes))
	fmt.Fprintln(os.Stderr, ui.OK.Render("✓ ")+"built "+imagePath)

	f, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(out, f); err != nil {
		return err
	}
	if file, ok := out.(*os.File); ok && file != os.Stdout {
		if info, err := file.Stat(); err == nil {
			fmt.Fprintln(os.Stderr, ui.OK.Render("✓ ")+"wrote "+label+" ("+project.FormatBytes(info.Size())+")")
		}
	}
	return nil
}

func dockerOutput(args []string) (io.Writer, string, func(), error) {
	if len(args) == 0 {
		if info, err := os.Stdout.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return nil, "", func() {}, fmt.Errorf("stdout is a terminal — use: lagoon docker image.tar")
		}
		return os.Stdout, "stdout", func() {}, nil
	}
	f, err := os.Create(args[0])
	if err != nil {
		return nil, "", func() {}, err
	}
	return f, args[0], func() { _ = f.Close() }, nil
}
