package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/spf13/cobra"
)

// sandboxPID is written to cacheDir/pid.json just before entering the sandbox.
type sandboxPID struct {
	PID      int      `json:"pid"`
	Project  string   `json:"project"`
	Packages []string `json:"packages"`
	Started  string   `json:"started"`
}

// writePIDFile records the current process's PID and project metadata.
func writePIDFile(cacheDir, project string, packages []string) {
	info := sandboxPID{
		PID:      os.Getpid(),
		Project:  project,
		Packages: packages,
		Started:  time.Now().Format(time.RFC3339),
	}
	b, _ := json.Marshal(info)
	_ = os.WriteFile(filepath.Join(cacheDir, "pid.json"), b, 0644)
}

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "show status and resource usage of sandboxes",
	RunE:  runPs,
}

func runPs(cmd *cobra.Command, args []string) error {
	// current project status (was status)
	cfg, err := config.Read(config.Filename)
	if err != nil {
		fmt.Println(warn("!") + " no lagoon.toml found — run 'lagoon init' first")
	} else {
		absPath, err := filepath.Abs(".")
		if err != nil {
			return err
		}
		cacheDir := projectCacheDir(absPath)
		shellNixPath := filepath.Join(cacheDir, "shell.nix")

		fmt.Println("  packages: " + strings.Join(cfg.Packages, " "))
		fmt.Println("  profile:  " + cfg.Profile)

		sum, _ := nix.GenerateShellNix(cfg, shellNixPath)
		if _, hit := nix.LoadCache(cacheDir, sum); hit {
			fmt.Println(ok("✓") + " cached — next 'lagoon shell' starts instantly")
		} else {
			fmt.Println(warn("!") + " not cached — run 'lagoon shell' to build")
		}
		fmt.Println()
	}

	// running sandbox processes (was stats — linux only)
	if runtime.GOOS != "linux" {
		fmt.Println(warn("!") + " sandbox process info is only available on Linux (/proc required)")
		return nil
	}

	lagoonCache := lagoonCacheBase()

	entries, err := filepath.Glob(filepath.Join(lagoonCache, "*/pid.json"))
	if err != nil || len(entries) == 0 {
		fmt.Println("  no sandboxes running")
		return nil
	}

	running := 0
	for _, pidFile := range entries {
		b, err := os.ReadFile(pidFile)
		if err != nil {
			continue
		}
		var info sandboxPID
		if err := json.Unmarshal(b, &info); err != nil {
			continue
		}
		if !isProcessAlive(info.PID) {
			continue
		}
		mem := readProcessMem(info.PID)
		pkgs := strings.Join(info.Packages, " ")
		fmt.Printf("  %s  pid %-6d  %-8s  %s\n", ok("●"), info.PID, mem, pkgs)
		fmt.Printf("     %s\n", info.Project)
		running++
	}

	if running == 0 {
		fmt.Println("  no sandboxes currently running")
	} else {
		fmt.Printf("\n  %d sandbox(es) running\n", running)
	}
	return nil
}

// isProcessAlive sends signal 0 to check if a process exists.
func isProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// readProcessMem reads VmRSS from /proc/<pid>/status and returns a formatted string.
func readProcessMem(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return "?"
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				kb, err := strconv.ParseInt(f[1], 10, 64)
				if err == nil {
					return fmt.Sprintf("%d MiB", kb/1024)
				}
			}
		}
	}
	return "?"
}
