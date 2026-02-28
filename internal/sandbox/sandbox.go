package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
)

// Enter replaces the current process with a bwrap sandbox.
// cmd is a one-off command to run; empty string opens an interactive shell.
// memory limits sandbox via systemd-run (e.g. "512m", "1g"); empty = no limit.
// extraEnvs are additional KEY=VALUE pairs injected into the sandbox.
func Enter(cfg *config.Config, env *nix.ResolvedEnv, projectPath, cmd, memory string, extraEnvs []string) error {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return fmt.Errorf("bwrap not found: %w", err)
	}

	bwrapArgs := buildArgs(cfg, env, projectPath, cmd, extraEnvs)

	if memory != "" {
		sysRun, err := exec.LookPath("systemd-run")
		if err != nil {
			return fmt.Errorf("--memory requires systemd-run (not found on this system): %w", err)
		}
		// wrap bwrap in a transient systemd scope with the memory limit
		argv := append([]string{"systemd-run", "--scope", "-p", "MemoryMax=" + strings.ToUpper(memory), "--", bwrap}, bwrapArgs...)
		return syscall.Exec(sysRun, argv, nil)
	}

	// sandbox env is set via --clearenv + --setenv inside buildArgs.
	// bwrap's own process env (third arg) doesn't affect the sandboxed shell.
	return syscall.Exec(bwrap, append([]string{"bwrap"}, bwrapArgs...), nil)
}

// buildArgs constructs the full bwrap argument list.
// order matters here — bwrap processes flags left to right.
func buildArgs(cfg *config.Config, env *nix.ResolvedEnv, projectPath, cmd string, extraEnvs []string) []string {
	args := []string{
		// nix store is read-only — packages live here
		"--ro-bind", "/nix/store", "/nix/store",

		// project directory mounted as /workspace
		"--bind", projectPath, "/workspace",

		// writable temp and home — ephemeral, gone when shell exits
		"--tmpfs", "/tmp",
		"--tmpfs", "/home",

		// create /etc so we can mount individual files into it
		"--dir", "/etc",

		// minimal /etc needed for dns, tls, and tools that check the user
		"--ro-bind-try", "/etc/resolv.conf", "/etc/resolv.conf",
		"--ro-bind-try", "/etc/ssl", "/etc/ssl",
		"--ro-bind-try", "/etc/ca-certificates", "/etc/ca-certificates",
		"--ro-bind-try", "/etc/passwd", "/etc/passwd",
		"--ro-bind-try", "/etc/group", "/etc/group",
		"--ro-bind-try", "/etc/nsswitch.conf", "/etc/nsswitch.conf",
		"--ro-bind-try", "/etc/localtime", "/etc/localtime",

		// symlinks so #!/bin/bash and #!/usr/bin/env both work
		"--symlink", env.BashPath, "/bin/sh",
		"--symlink", env.BashPath, "/bin/bash",
		"--symlink", env.EnvPath, "/usr/bin/env",

		// /proc and /dev are needed by most programs
		"--proc", "/proc",
		"--dev", "/dev",

		// drop all host namespaces
		"--unshare-all",

		// kill sandbox if lagoon dies (shouldn't happen with syscall.Exec, but be safe)
		"--die-with-parent",

		// wipe inherited env — we'll set exactly what we need via --setenv
		"--clearenv",
		"--setenv", "HOME", "/home",
		"--setenv", "PATH", env.PATH,
		"--setenv", "TERM", os.Getenv("TERM"),
		"--setenv", "USER", os.Getenv("USER"),
		"--setenv", "LANG", "C.UTF-8",
		// show [lagoon] prefix in prompt so users know they're in the sandbox
		"--setenv", "PS1", "[lagoon] \\w $ ",

		// start in the project directory
		"--chdir", "/workspace",
	}

	// network is off by default — only add --share-net for the "network" profile
	if cfg.Profile == "network" {
		args = append(args, "--share-net")
	}

	// inject caller-provided env vars (KEY=VALUE)
	for _, kv := range extraEnvs {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			args = append(args, "--setenv", parts[0], parts[1])
		}
	}

	// launch an interactive shell, or run the given command non-interactively
	if cmd != "" {
		args = append(args, "--", env.BashPath, "-c", cmd)
	} else {
		args = append(args, "--", env.BashPath)
	}

	return args
}
