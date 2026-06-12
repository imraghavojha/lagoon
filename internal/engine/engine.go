// Package engine drives a container runtime on macOS. Lagoon prefers
// apple/container (lightweight VMs, very fast boot on Apple Silicon) and
// falls back to Docker when it isn't installed. On Linux, Lagoon uses
// Nix + bubblewrap instead — see internal/sandbox.
package engine

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/imraghavojha/lagoon/internal/config"
)

// Kind identifies which container runtime backs the engine.
type Kind int

const (
	AppleContainer Kind = iota
	Docker
)

// Engine wraps the container CLI lagoon shells out to.
type Engine struct {
	Bin  string // resolved binary path
	Kind Kind
}

// Name is the human-readable engine name shown in dashboards.
func (e *Engine) Name() string {
	if e.Kind == AppleContainer {
		return "apple/container"
	}
	return "docker"
}

// UseContainers reports whether this platform runs environments in containers.
func UseContainers() bool {
	return runtime.GOOS == "darwin"
}

// Detect finds a container runtime: apple/container first, then docker.
func Detect() (*Engine, error) {
	if bin, err := exec.LookPath("container"); err == nil {
		return &Engine{Bin: bin, Kind: AppleContainer}, nil
	}
	if bin, err := exec.LookPath("docker"); err == nil {
		return &Engine{Bin: bin, Kind: Docker}, nil
	}
	return nil, fmt.Errorf(`no container runtime found.
  install apple/container (recommended, fastest on Apple Silicon):
    brew install container && container system start
  or install Docker Desktop: https://docker.com`)
}

// EnsureRunning verifies the runtime daemon/apiserver is up, starting
// apple/container's service automatically when possible.
func (e *Engine) EnsureRunning() error {
	switch e.Kind {
	case AppleContainer:
		if e.apiServerRunning() {
			return nil
		}
		// idempotent and fast when the kernel is already installed
		start := exec.Command(e.Bin, "system", "start")
		start.Stdin = strings.NewReader("Y\n")
		if out, err := start.CombinedOutput(); err != nil {
			return fmt.Errorf("could not start apple/container services:\n%s  run manually: container system start", string(out))
		}
		return nil
	default:
		if err := exec.Command(e.Bin, "info").Run(); err != nil {
			return fmt.Errorf("docker daemon is not running — start Docker Desktop and retry")
		}
		return nil
	}
}

func (e *Engine) apiServerRunning() bool {
	out, err := exec.Command(e.Bin, "system", "status").Output()
	return err == nil && strings.Contains(string(out), "running")
}

// presetImages maps lagoon presets to small official images. The lagoon.toml
// stays portable: Linux machines resolve the same file through Nix.
var presetImages = map[string]string{
	"python":  "python:3.12-slim",
	"node":    "node:22-slim",
	"go":      "golang:1.24",
	"llama":   "debian:stable-slim",
	"whisper": "debian:stable-slim",
	"custom":  "alpine:3.21",
}

// ImageFor resolves the container image for a config: an explicit image wins,
// then the preset, then a sniff of the package list, then alpine.
func ImageFor(cfg *config.Config) string {
	if cfg.Image != "" {
		return cfg.Image
	}
	if img, ok := presetImages[cfg.Preset]; ok {
		return img
	}
	for _, pkg := range cfg.Packages {
		switch {
		case strings.HasPrefix(pkg, "python"):
			return presetImages["python"]
		case strings.HasPrefix(pkg, "nodejs"):
			return presetImages["node"]
		case pkg == "go":
			return presetImages["go"]
		}
	}
	return presetImages["custom"]
}

// shellFor picks the interactive shell available in the image.
func shellFor(image string) string {
	if strings.HasPrefix(image, "alpine") {
		return "sh"
	}
	return "bash"
}

// HasImage reports whether the image is already present locally (warm start).
func (e *Engine) HasImage(image string) bool {
	if e.Kind == AppleContainer {
		return exec.Command(e.Bin, "image", "inspect", image).Run() == nil
	}
	return exec.Command(e.Bin, "image", "inspect", image).Run() == nil
}

// Pull fetches the image, streaming CLI progress to the writers the caller set.
func (e *Engine) Pull(image string) *exec.Cmd {
	if e.Kind == AppleContainer {
		return exec.Command(e.Bin, "image", "pull", image)
	}
	return exec.Command(e.Bin, "pull", image)
}

// ShellArgs builds the argv (excluding argv[0]) for an interactive shell or
// one-off command in a container, mirroring the bwrap sandbox semantics:
// project mounted at /workspace, ephemeral container, scoped env.
func ShellArgs(e *Engine, cfg *config.Config, projectPath, cmd, memory string, extraEnvs []string) []string {
	image := ImageFor(cfg)
	sh := shellFor(image)

	args := []string{"run", "--rm",
		"--volume", projectPath + ":/workspace",
		"--workdir", "/workspace",
		"--env", `PS1=[lagoon] \w $ `,
		"--env", "LANG=C.UTF-8",
	}
	if cmd == "" {
		args = append(args, "-it")
	} else {
		args = append(args, "-i")
	}
	if memory != "" {
		args = append(args, "--memory", strings.ToUpper(memory))
	}
	for _, kv := range extraEnvs {
		args = append(args, "--env", kv)
	}
	args = append(args, image)

	// interactive bash runs --norc so the PS1 we inject isn't overridden by
	// the image's /etc/bash.bashrc — the [lagoon] prompt matches Linux.
	// no `bash -c` wrapper for the plain case: non-interactive bash unsets
	// PS1, which would kill the prompt before the inner shell starts.
	switch {
	case cmd != "" && cfg.OnEnter != "":
		args = append(args, sh, "-c", cfg.OnEnter+" && "+cmd)
	case cmd != "":
		args = append(args, sh, "-c", cmd)
	case cfg.OnEnter != "" && sh == "bash":
		args = append(args, sh, "-c", cfg.OnEnter+`; export PS1='[lagoon] \w $ '; exec bash --norc`)
	case cfg.OnEnter != "":
		args = append(args, sh, "-c", cfg.OnEnter+"; PS1='[lagoon] $ ' exec "+sh)
	case sh == "bash":
		args = append(args, "bash", "--norc")
	default:
		args = append(args, sh)
	}
	return args
}

// ServiceArgs builds argv for one [up] service: attached (so logs stream and
// SIGTERM stops it), named for discoverability, with inferred ports published.
func ServiceArgs(e *Engine, cfg *config.Config, projectPath, name, command, memory string, ports []string) []string {
	image := ImageFor(cfg)
	sh := shellFor(image)

	args := []string{"run", "--rm",
		"--name", ContainerName(projectPath, name),
		"--volume", projectPath + ":/workspace",
		"--workdir", "/workspace",
		"--env", "LANG=C.UTF-8",
	}
	if memory != "" {
		args = append(args, "--memory", strings.ToUpper(memory))
	}
	for _, p := range ports {
		args = append(args, "--publish", p+":"+p)
	}
	args = append(args, image, sh, "-c", command)
	return args
}

// ContainerName returns a stable, unique container name for a project service.
func ContainerName(projectPath, service string) string {
	base := strings.ToLower(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '-'
	}, lastPathElement(projectPath)))
	return "lagoon-" + strings.Trim(base, "-") + "-" + service
}

func lastPathElement(p string) string {
	p = strings.TrimRight(p, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// RemoveContainer force-removes a container by name; best-effort cleanup.
func (e *Engine) RemoveContainer(name string) {
	if e.Kind == AppleContainer {
		_ = exec.Command(e.Bin, "rm", "-f", name).Run()
		return
	}
	_ = exec.Command(e.Bin, "rm", "-f", name).Run()
}

// SaveImage exports an image as an OCI/Docker-compatible tar archive.
func (e *Engine) SaveImage(image, outFile string) *exec.Cmd {
	if e.Kind == AppleContainer {
		return exec.Command(e.Bin, "image", "save", "--output", outFile, image)
	}
	return exec.Command(e.Bin, "save", "--output", outFile, image)
}

// LoadImage imports an image tar archive.
func (e *Engine) LoadImage(inFile string) *exec.Cmd {
	if e.Kind == AppleContainer {
		return exec.Command(e.Bin, "image", "load", "--input", inFile)
	}
	return exec.Command(e.Bin, "load", "--input", inFile)
}

// BootNote describes the speed story for the active engine, shown in dashboards.
func (e *Engine) BootNote() string {
	if e.Kind == AppleContainer {
		return "sub-second VM boot via apple/container"
	}
	return "running via Docker"
}

// WaitHealthy polls until a named container reports running, with timeout.
// Used by tests; commands rely on attached processes instead.
func (e *Engine) WaitHealthy(name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command(e.Bin, "inspect", name).Output()
		if err == nil && strings.Contains(string(out), "running") {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
