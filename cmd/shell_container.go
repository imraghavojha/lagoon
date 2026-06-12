package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/engine"
	"github.com/imraghavojha/lagoon/internal/hardware"
	"github.com/imraghavojha/lagoon/internal/ui"
)

// runShellContainer is the macOS path for `lagoon shell` and `lagoon run`:
// the environment runs in a lightweight VM container (apple/container) or
// docker, with the project mounted at /workspace, mirroring the Linux sandbox.
func runShellContainer(cfg *config.Config) error {
	eng, err := engine.Detect()
	if err != nil {
		fmt.Fprintln(os.Stderr, fail("✗")+" "+err.Error())
		os.Exit(1)
	}

	absPath, err := filepath.Abs(".")
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	image := engine.ImageFor(cfg)
	warm := eng.HasImage(image)
	if !warm {
		fmt.Println(ui.Hot.Render("↓") + " pulling " + image + " (first run only)")
		pull := eng.Pull(image)
		pull.Stdout = os.Stdout
		pull.Stderr = os.Stderr
		if err := pull.Run(); err != nil {
			return fmt.Errorf("pulling %s: %w", image, err)
		}
	}

	effectiveMem := memFlag
	if effectiveMem == "" {
		effectiveMem = cfg.MemoryCap
	}
	renderContainerShellHeader(cfg, eng, image, warm, effectiveMem)

	// record pid so 'lagoon ps' can find this sandbox (same pid after syscall.Exec)
	writePIDFile(projectCacheDir(absPath), absPath, cfg.Packages)

	argv := append([]string{filepath.Base(eng.Bin)},
		engine.ShellArgs(eng, cfg, absPath, cmdFlag, effectiveMem, envFlags)...)
	return syscall.Exec(eng.Bin, argv, os.Environ())
}

func renderContainerShellHeader(cfg *config.Config, eng *engine.Engine, image string, warm bool, memory string) {
	machine := hardware.Detect(".")
	if memory == "" {
		memory = "none"
	}
	cache := "cold→warm"
	if warm {
		cache = "warm"
	}
	lines := []string{
		ui.Chip("machine", string(machine.Class), ui.Accent) + "  " + ui.Chip("arch", machine.Arch, ui.Accent) + "  " + ui.Chip("cores", fmt.Sprint(machine.Cores), ui.Accent),
		ui.Chip("engine", eng.Name(), ui.Good) + "  " + ui.Chip("image", image, ui.Accent),
		ui.Chip("memory cap", strings.ToUpper(memory), ui.Warn) + "  " + ui.Chip("cache", cache, ui.Good),
	}
	if cmdFlag == "" {
		lines = append(lines, ui.Bullet(ui.Dim.Render("•"), "type exit to return to host shell"))
	}
	fmt.Println("\n" + ui.Card("Lagoon shell", lines...) + "\n")
}
