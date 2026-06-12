package cmd

import (
	"testing"

	"github.com/imraghavojha/lagoon/internal/hardware"
)

func TestConfigFromChoicesUsesPresetAndExtras(t *testing.T) {
	cfg, err := configFromChoices(initChoices{
		Intent:   intentDev,
		PresetID: "python",
		Packages: []string{"git", "uv"},
		Network:  true,
	}, hardware.Machine{Class: hardware.LaptopClass})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"python311", "uv", "git"}
	if len(cfg.Packages) != len(want) {
		t.Fatalf("packages = %v", cfg.Packages)
	}
	for i := range want {
		if cfg.Packages[i] != want[i] {
			t.Fatalf("packages = %v, want %v", cfg.Packages, want)
		}
	}
	if cfg.Profile != "network" || cfg.Intent != intentDev || cfg.Preset != "python" || cfg.MemoryCap != "2g" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestConfigFromChoicesHonorsNetworkOff(t *testing.T) {
	// the user explicitly turned network off for a network-profile preset —
	// the answer must win over the preset default
	cfg, err := configFromChoices(initChoices{
		Intent:   intentDev,
		PresetID: "python", // preset profile is "network"
		Network:  false,
	}, hardware.Machine{Class: hardware.LaptopClass})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "minimal" {
		t.Fatalf("profile = %q, want minimal (explicit network off must win)", cfg.Profile)
	}
}

func TestConfigFromChoicesServiceAddsUp(t *testing.T) {
	cfg, err := configFromChoices(initChoices{
		Intent:     intentServices,
		PresetID:   "go",
		ServiceCmd: "go run .",
	}, hardware.Machine{Class: hardware.PiClass})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Up["app"] != "go run ." {
		t.Fatalf("up = %+v", cfg.Up)
	}
	if cfg.MemoryCap != "768m" {
		t.Fatalf("memory cap = %q", cfg.MemoryCap)
	}
}

func TestDefaultServiceCommand(t *testing.T) {
	if got := defaultServiceCommand("node"); got == "" {
		t.Fatal("node service default should not be empty")
	}
	if got := defaultServiceCommand("custom"); got != "" {
		t.Fatalf("custom default = %q", got)
	}
}
