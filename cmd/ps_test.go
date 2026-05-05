package cmd

import (
	"strings"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/hardware"
	"github.com/imraghavojha/lagoon/internal/project"
)

func TestIncludeEntryForProjectDefaultsCurrentOnly(t *testing.T) {
	entry := sandboxPID{Project: "/work/other"}
	if includeEntryForProject(entry, "/work/current", false) {
		t.Fatal("default ps scope should exclude other projects")
	}
	if !includeEntryForProject(entry, "/work/current", true) {
		t.Fatal("--all should include other projects")
	}
	if !includeEntryForProject(sandboxPID{Project: "/work/current"}, "/work/current", false) {
		t.Fatal("default ps scope should include current project")
	}
}

func TestRenderStatusDashboardShowsConfiguredServicePorts(t *testing.T) {
	cfg := &config.Config{
		Packages:  []string{"python311"},
		Profile:   "network",
		MemoryCap: "2g",
		Up:        map[string]string{"web": "python3 -m http.server 8000"},
	}
	out := renderStatusDashboard(cfg, project.Status{ProjectPath: "/work/current"}, hardware.Machine{Class: hardware.LaptopClass, Arch: "amd64", Cores: 4, TotalRAMMiB: 8192}, nil, false)
	if !strings.Contains(out, "ports:") || !strings.Contains(out, "8000") {
		t.Fatalf("dashboard should include configured service ports, got:\n%s", out)
	}
	if !strings.Contains(out, "nothing running for this project") {
		t.Fatalf("dashboard should use current-project empty state, got:\n%s", out)
	}
}

func TestRenderStatusDashboardShowsRunningPortsAndAllProjectLabel(t *testing.T) {
	cfg := &config.Config{Packages: []string{"nodejs_22"}, Profile: "network", Up: map[string]string{"web": "PORT=3000 node server.js"}}
	entries := []sandboxPID{{PID: 123, Project: "/work/other", Kind: "service", Name: "web", Command: "PORT=3000 node server.js", Ports: []string{"3000"}}}
	out := renderStatusDashboard(cfg, project.Status{ProjectPath: "/work/current", CacheReady: true}, hardware.Machine{Class: hardware.LaptopClass, Arch: "amd64", Cores: 4}, entries, true)
	if !strings.Contains(out, "ports:3000") || !strings.Contains(out, "/work/other") {
		t.Fatalf("dashboard should include running ports and project label for --all, got:\n%s", out)
	}
}
