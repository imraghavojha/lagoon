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

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "show memory usage of running sandbox processes",
	RunE:  runStats,
}

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

func runStats(cmd *cobra.Command, args []string) error {
	if runtime.GOOS != "linux" {
		fmt.Println(warn("!") + " lagoon stats is only available on Linux (/proc required)")
		return nil
	}

	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	lagoonCache := filepath.Join(base, "lagoon")

	entries, err := filepath.Glob(filepath.Join(lagoonCache, "*/pid.json"))
	if err != nil || len(entries) == 0 {
		fmt.Println("  no sandboxes found")
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

		alive := isProcessAlive(info.PID)
		if !alive {
			continue
		}

		mem := readProcessMem(info.PID)
		pkgs := strings.Join(info.Packages, " ")
		fmt.Printf("  %s  pid %-6d  %-8s  %s\n",
			ok("●"), info.PID, mem, pkgs)
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
