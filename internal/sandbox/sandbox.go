package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/kuldeepojha/lagoon/internal/config"
	"github.com/kuldeepojha/lagoon/internal/nix"
)

// Enter replaces the current process with a bwrap sandbox.
// when the user exits the shell, they're back on the host — no cleanup needed.
func Enter(cfg *config.Config, env *nix.ResolvedEnv, projectPath string) error {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return fmt.Errorf("bwrap not found: %w", err)
	}

	args := buildArgs(cfg, env, projectPath)

	// grab TERM and USER from the host before we build the clean env slice
	term := os.Getenv("TERM")
	user := os.Getenv("USER")

	// only pass what the sandbox needs — everything else leaks host state
	sandboxEnv := []string{
		"HOME=/home",
		"PATH=" + env.PATH,
		"TERM=" + term,
		"USER=" + user,
		"LANG=C.UTF-8",
	}

	// syscall.Exec replaces this process image entirely
	return syscall.Exec(bwrap, append([]string{"bwrap"}, args...), sandboxEnv)
}

// buildArgs constructs the full bwrap argument list.
// order matters here — bwrap processes flags left to right.
func buildArgs(cfg *config.Config, env *nix.ResolvedEnv, projectPath string) []string {
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

		// start in the project directory
		"--chdir", "/workspace",
	}

	// network is off by default — only add --share-net for the "network" profile
	if cfg.Profile == "network" {
		args = append(args, "--share-net")
	}

	// the shell to launch inside the sandbox
	args = append(args, "--", env.BashPath)

	return args
}
