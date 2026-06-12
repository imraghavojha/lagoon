package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/imraghavojha/lagoon/internal/engine"
	"github.com/imraghavojha/lagoon/internal/project"
	"github.com/imraghavojha/lagoon/internal/ui"
	"github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
	Use:   "load <file>",
	Short: "load an offline Lagoon environment archive",
	Args:  cobra.ExactArgs(1),
	RunE:  runLoad,
}

func runLoad(cmd *cobra.Command, args []string) error {
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()
	info, _ := f.Stat()
	size := "unknown"
	if info != nil {
		size = project.FormatBytes(info.Size())
	}

	// macOS: archives are OCI image tars — import them through the engine
	if engine.UseContainers() {
		eng, err := engine.Detect()
		if err != nil {
			return err
		}
		if err := eng.EnsureRunning(); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, ui.Card("Lagoon load",
			ui.Chip("engine", eng.Name(), ui.Good),
			ui.Chip("source", args[0], ui.Accent),
			ui.Chip("archive", size, ui.Good),
		))
		imp := eng.LoadImage(args[0])
		imp.Stdout = os.Stdout
		imp.Stderr = os.Stderr
		if err := imp.Run(); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, ui.OK.Render("✓ ")+"loaded "+args[0])
		return nil
	}

	fmt.Fprintln(os.Stderr, ui.Card("Lagoon load",
		ui.Chip("source", args[0], ui.Accent),
		ui.Chip("archive", size, ui.Good),
		ui.Bullet(ui.Dim.Render("•"), "importing Nix store paths for offline use"),
	))

	imp := exec.Command("nix-store", "--import")
	imp.Stdin = f
	imp.Stdout = os.Stdout
	imp.Stderr = os.Stderr
	if err := imp.Run(); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, ui.OK.Render("✓ ")+"loaded "+args[0])
	return nil
}
