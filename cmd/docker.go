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
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "export the environment as a Docker image tar",
	Long: `lagoon docker > myimage.tar

Builds a layered Docker image from the nix environment defined in lagoon.toml.
The resulting tar can be loaded with:  docker load < myimage.tar
Uses nixpkgs.dockerTools.buildLayeredImage — no Docker daemon required to build.`,
	RunE: runDocker,
}

func runDocker(cmd *cobra.Command, args []string) error {
	// refuse to dump binary tar data to a terminal
	if info, err := os.Stdout.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return fmt.Errorf("stdout is a terminal — redirect to a file: lagoon docker > myimage.tar")
	}

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

	fmt.Fprintln(os.Stderr, warn("→")+" building docker image "+name+":latest (first run may be slow)…")

	// nix-build --no-out-link prints the store path of the built image tar to stdout
	build := exec.Command("nix-build", "--no-out-link", dockerNixPath)
	build.Stderr = os.Stderr
	outBytes, err := build.Output()
	if err != nil {
		return fmt.Errorf("nix-build failed: %w", err)
	}

	imagePath := strings.TrimSpace(string(outBytes))
	fmt.Fprintln(os.Stderr, ok("✓")+" built "+imagePath)

	f, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(os.Stdout, f)
	return err
}
