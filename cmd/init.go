package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/hardware"
	"github.com/imraghavojha/lagoon/internal/presets"
	"github.com/imraghavojha/lagoon/internal/ui"
	"github.com/spf13/cobra"
)

const (
	intentDev      = "dev-workspace"
	intentServices = "service-stack"
	intentPortable = "portable-runtime"
)

type initChoices struct {
	Intent     string
	PresetID   string
	Packages   []string
	Network    bool
	ServiceCmd string
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "create a hardware-aware lagoon.toml in the current directory",
	Long: `lagoon init

Detects this machine, asks what you want Lagoon to do, offers curated presets,
and previews the final portable config before writing lagoon.toml. Detection
only guides defaults — every choice remains yours.`,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	machine := hardware.Detect(".")
	fmt.Println(renderInitHero(machine))

	if _, err := os.Stat(config.Filename); err == nil {
		var overwrite bool
		if err := huh.NewConfirm().
			Title("lagoon.toml already exists. overwrite?").
			Affirmative("overwrite").
			Negative("keep existing").
			Value(&overwrite).
			Run(); err != nil {
			return err
		}
		if !overwrite {
			fmt.Println(ui.Dim.Render("kept existing lagoon.toml"))
			return nil
		}
	}

	choices, err := collectInitChoices(machine)
	if err != nil {
		return err
	}
	cfg, err := configFromChoices(choices, machine)
	if err != nil {
		return err
	}

	fmt.Println(renderInitPreview(cfg, machine))
	var confirm bool
	if err := huh.NewConfirm().
		Title("write this lagoon.toml?").
		Affirmative("write config").
		Negative("cancel").
		Value(&confirm).
		Run(); err != nil {
		return err
	}
	if !confirm {
		fmt.Println(ui.Dim.Render("not written"))
		return nil
	}

	if err := config.Write(config.Filename, cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Println(ui.Card("Lagoon is ready",
		ui.Bullet(ui.OK.Render("✓"), "created lagoon.toml"),
		ui.Bullet(ui.Hot.Render("→"), nextCommandForIntent(cfg.Intent)),
		ui.Bullet(ui.Dim.Render("•"), "commit lagoon.toml so teammates get the same environment"),
	))
	return nil
}

func collectInitChoices(machine hardware.Machine) (initChoices, error) {
	choices := initChoices{Intent: intentDev, PresetID: presets.NoneID}

	intentOptions := []huh.Option[string]{
		huh.NewOption("Dev Workspace — instant reproducible shells", intentDev),
		huh.NewOption("Service Stack — small localhost services", intentServices),
		huh.NewOption("Portable Runtime — save/load for offline machines", intentPortable),
	}
	presetOptions := make([]huh.Option[string], 0, len(presets.All))
	for _, p := range presets.All {
		label := p.Name
		if !presets.SafeForRAM(p, machine.TotalRAMMiB) {
			label += "  (heavy for this machine)"
		}
		presetOptions = append(presetOptions, huh.NewOption(label+" — "+p.Description, p.ID))
	}

	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What are you building first?").
				Description(machineGuidance(machine)).
				Options(intentOptions...).
				Value(&choices.Intent),
			huh.NewSelect[string]().
				Title("Pick a starting preset").
				Description("Curated presets are small on purpose. You can add packages next.").
				Options(presetOptions...).
				Value(&choices.PresetID),
		),
	).Run(); err != nil {
		return choices, err
	}

	preset, _ := presets.Find(choices.PresetID)
	choices.Network = preset.Profile == "network" || choices.Intent == intentServices
	if err := huh.NewConfirm().
		Title("network access inside the sandbox?").
		Description("Network is off by default for portable/offline runtimes; services always use host networking when running.").
		Affirmative("network on").
		Negative("network off").
		Value(&choices.Network).
		Run(); err != nil {
		return choices, err
	}

	var addPackages bool
	if err := huh.NewConfirm().
		Title("add more packages from nixpkgs search?").
		Description("Press Esc in the search screen when done. You can also edit lagoon.toml later.").
		Affirmative("search packages").
		Negative("use preset only").
		Value(&addPackages).
		Run(); err != nil {
		return choices, err
	}
	if addPackages {
		pkgs, err := RunPackageSearch()
		if err != nil {
			return choices, err
		}
		choices.Packages = pkgs
	}

	if choices.Intent == intentServices {
		choices.ServiceCmd = defaultServiceCommand(choices.PresetID)
		if err := huh.NewInput().
			Title("Service command for lagoon up").
			Description("One small localhost process to start with. Leave empty to skip [up].").
			Placeholder("python3 -m http.server 8000").
			Value(&choices.ServiceCmd).
			Run(); err != nil {
			return choices, err
		}
	}

	return choices, nil
}

func configFromChoices(choices initChoices, machine hardware.Machine) (*config.Config, error) {
	preset, ok := presets.Find(choices.PresetID)
	if !ok {
		return nil, fmt.Errorf("unknown preset: %s", choices.PresetID)
	}
	packages := presets.MergePackages(preset.Packages, choices.Packages)
	if len(packages) == 0 {
		packages = []string{"bashInteractive", "coreutils"}
	}
	profile := "minimal"
	if choices.Network || preset.Profile == "network" && choices.Intent == intentDev {
		profile = "network"
	}
	cfg := &config.Config{
		Packages:      packages,
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       profile,
		Intent:        choices.Intent,
		Preset:        choices.PresetID,
		MemoryCap:     hardware.DefaultMemoryCap(machine),
	}
	if choices.Intent == intentServices && strings.TrimSpace(choices.ServiceCmd) != "" {
		cfg.Up = map[string]string{"app": strings.TrimSpace(choices.ServiceCmd)}
	}
	return cfg, nil
}

func renderInitHero(machine hardware.Machine) string {
	lines := []string{
		ui.Chip("machine", string(machine.Class), ui.Accent),
		ui.Chip("ram", hardware.FormatMiB(machine.TotalRAMMiB), ui.Good) + "  " + ui.Chip("cap", hardware.DefaultMemoryCap(machine), ui.Warn),
		ui.Chip("arch", machine.Arch, ui.Accent) + "  " + ui.Chip("cores", fmt.Sprint(machine.Cores), ui.Accent),
		ui.Chip("disk free", hardware.FormatMiB(machine.DiskFreeMiB), ui.Good),
	}
	for _, warning := range hardware.Warnings(machine) {
		lines = append(lines, ui.Bullet(ui.Hot.Render("!"), warning))
	}
	return ui.Card("Lagoon init", lines...)
}

func renderInitPreview(cfg *config.Config, machine hardware.Machine) string {
	cacheState := "cold"
	lines := []string{
		ui.Chip("intent", displayIntent(cfg.Intent), ui.Accent),
		ui.Chip("preset", cfg.Preset, ui.Accent),
		ui.Chip("packages", strings.Join(cfg.Packages, ", "), ui.Good),
		ui.Chip("network", onOff(cfg.Profile == "network"), ui.Warn),
		ui.Chip("memory cap", cfg.MemoryCap, ui.Warn),
		ui.Chip("cache", cacheState+" first run", ui.Warn),
		ui.Chip("machine", machine.Summary(), ui.Accent),
	}
	if len(cfg.Up) > 0 {
		lines = append(lines, ui.Chip("lagoon up", cfg.Up["app"], ui.Good))
	}
	return ui.Card("Preview lagoon.toml", lines...)
}

func machineGuidance(machine hardware.Machine) string {
	if hardware.UpRecommended(machine) {
		return "Detected " + machine.Summary() + ". Service stacks should be fine if packages stay small."
	}
	return "Detected " + machine.Summary() + ". Shells and portable runtimes are safest; services still work if kept tiny."
}

func defaultServiceCommand(presetID string) string {
	switch presetID {
	case "python":
		return "python3 -m http.server 8000"
	case "node":
		return "pnpm dev --host 0.0.0.0"
	case "go":
		return "go run ."
	default:
		return ""
	}
}

func displayIntent(intent string) string {
	switch intent {
	case intentServices:
		return "Service Stack"
	case intentPortable:
		return "Portable Runtime"
	default:
		return "Dev Workspace"
	}
}

func nextCommandForIntent(intent string) string {
	switch intent {
	case intentServices:
		return "next: lagoon up"
	case intentPortable:
		return "next: lagoon shell, then lagoon save > runtime.nar"
	default:
		return "next: lagoon shell"
	}
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}
