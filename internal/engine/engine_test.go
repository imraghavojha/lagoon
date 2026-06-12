package engine

import (
	"strings"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
)

func TestImageFor(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.Config
		want string
	}{
		{"explicit image wins", config.Config{Image: "ubuntu:24.04", Preset: "python"}, "ubuntu:24.04"},
		{"python preset", config.Config{Preset: "python"}, "python:3.12-slim"},
		{"node preset", config.Config{Preset: "node"}, "node:22-slim"},
		{"go preset", config.Config{Preset: "go"}, "golang:1.24"},
		{"package sniff python", config.Config{Packages: []string{"python311", "uv"}}, "python:3.12-slim"},
		{"package sniff node", config.Config{Packages: []string{"nodejs_22"}}, "node:22-slim"},
		{"package sniff go", config.Config{Packages: []string{"go"}}, "golang:1.24"},
		{"fallback alpine", config.Config{Packages: []string{"ripgrep"}}, "alpine:3.21"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ImageFor(&tc.cfg); got != tc.want {
				t.Errorf("ImageFor() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestContainerName(t *testing.T) {
	got := ContainerName("/Users/me/My App", "web")
	if got != "lagoon-my-app-web" {
		t.Errorf("ContainerName() = %q", got)
	}
}

func TestShellArgs(t *testing.T) {
	e := &Engine{Bin: "container", Kind: AppleContainer}
	cfg := &config.Config{Preset: "python", MemoryCap: "2g"}

	args := ShellArgs(e, cfg, "/proj", "", "2g", []string{"FOO=bar"})
	joined := strings.Join(args, " ")
	for _, want := range []string{"run", "--rm", "-it", "/proj:/workspace", "--workdir /workspace", "--memory 2G", "--env FOO=bar", "python:3.12-slim"} {
		if !strings.Contains(joined, want) {
			t.Errorf("ShellArgs missing %q in %q", want, joined)
		}
	}
	if args[len(args)-1] != "bash" {
		t.Errorf("interactive shell should end with bash, got %q", args[len(args)-1])
	}

	// one-off command goes through sh -c
	args = ShellArgs(e, cfg, "/proj", "echo hi", "", nil)
	if args[len(args)-1] != "echo hi" || args[len(args)-2] != "-c" {
		t.Errorf("one-off command not passed via -c: %v", args[len(args)-3:])
	}

	// on_enter hook runs before the command
	cfg.OnEnter = "pip install -q -r requirements.txt"
	args = ShellArgs(e, cfg, "/proj", "pytest", "", nil)
	if got := args[len(args)-1]; got != "pip install -q -r requirements.txt && pytest" {
		t.Errorf("on_enter chain wrong: %q", got)
	}
}

func TestServiceArgs(t *testing.T) {
	e := &Engine{Bin: "container", Kind: AppleContainer}
	cfg := &config.Config{Preset: "python"}
	args := ServiceArgs(e, cfg, "/Users/me/proj", "app", "python3 -m http.server 8000", "1g", []string{"8000"})
	joined := strings.Join(args, " ")
	for _, want := range []string{"--name lagoon-proj-app", "--publish 8000:8000", "--memory 1G", "python3 -m http.server 8000"} {
		if !strings.Contains(joined, want) {
			t.Errorf("ServiceArgs missing %q in %q", want, joined)
		}
	}
}
