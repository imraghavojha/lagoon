package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/engine"
	"github.com/imraghavojha/lagoon/internal/hardware"
	"github.com/imraghavojha/lagoon/internal/project"
	"github.com/imraghavojha/lagoon/internal/ui"
	"github.com/spf13/cobra"
)

// sandboxPID is written to cacheDir/*.pid.json for shells and services.
type sandboxPID struct {
	PID      int      `json:"pid"`
	Project  string   `json:"project"`
	Packages []string `json:"packages"`
	Started  string   `json:"started"`
	Kind     string   `json:"kind,omitempty"`
	Name     string   `json:"name,omitempty"`
	Command  string   `json:"command,omitempty"`
	Ports    []string `json:"ports,omitempty"`
}

// writePIDFile records the current process's PID and project metadata.
func writePIDFile(cacheDir, project string, packages []string) {
	writeProcessFile(filepath.Join(cacheDir, "pid.json"), sandboxPID{
		PID:      os.Getpid(),
		Project:  project,
		Packages: packages,
		Started:  time.Now().Format(time.RFC3339),
		Kind:     "shell",
		Name:     "shell",
	})
}

func writeServicePIDFile(cacheDir, project string, cfg *config.Config, name, command string, pid int) {
	writeProcessFile(filepath.Join(cacheDir, "service-"+safePIDName(name)+".pid.json"), sandboxPID{
		PID:      pid,
		Project:  project,
		Packages: cfg.Packages,
		Started:  time.Now().Format(time.RFC3339),
		Kind:     "service",
		Name:     name,
		Command:  command,
		Ports:    inferPorts(command),
	})
}

func writeProcessFile(path string, info sandboxPID) {
	// the cache dir may not exist yet on the container backend (no shell.nix)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	b, _ := json.Marshal(info)
	_ = os.WriteFile(path, b, 0644)
}

func safePIDName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "service"
	}
	return b.String()
}

var psAllProjects bool

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "show the Lagoon status dashboard",
	RunE:  runPs,
}

func init() {
	psCmd.Flags().BoolVar(&psAllProjects, "all", false, "show running Lagoon processes from all projects")
}

func runPs(cmd *cobra.Command, args []string) error {
	machine := hardware.Detect(".")
	absPath, _ := filepath.Abs(".")
	var cfg *config.Config
	var status project.Status
	cfg, err := config.Read(config.Filename)
	if err == nil {
		status = project.Inspect(cfg, absPath, lagoonCacheBase())
		// macOS: "warm" means the container image is already local
		if engine.UseContainers() {
			status.CacheReady = false
			if eng, eerr := engine.Detect(); eerr == nil {
				status.CacheReady = eng.HasImage(engine.ImageFor(cfg))
			}
		}
	}

	entries := runningEntries(lagoonCacheBase(), absPath, psAllProjects)
	fmt.Println(renderStatusDashboard(cfg, status, machine, entries, psAllProjects))
	return nil
}

func renderStatusDashboard(cfg *config.Config, status project.Status, machine hardware.Machine, entries []sandboxPID, allProjects bool) string {
	if cfg == nil {
		return ui.Card("Lagoon ps",
			ui.Bullet(ui.Hot.Render("!"), "no lagoon.toml found"),
			ui.Bullet(ui.Dim.Render("•"), "run lagoon init to create a reproducible environment"),
			ui.Chip("machine", machine.Summary(), ui.Accent),
		)
	}

	cacheColor := ui.Warn
	cacheLabel := "cold"
	if status.CacheReady {
		cacheColor = ui.Good
		cacheLabel = "warm"
	}
	network := cfg.Profile == "network"
	memoryCap := cfg.MemoryCap
	if memoryCap == "" {
		memoryCap = "none"
	}
	serviceCount := 0
	shellCount := 0
	for _, e := range entries {
		if e.Kind == "service" {
			serviceCount++
		} else {
			shellCount++
		}
	}

	lines := []string{
		ui.Chip("machine", string(machine.Class), ui.Accent) + "  " + ui.Chip("ram", hardware.FormatMiB(machine.TotalRAMMiB), ui.Good) + "  " + ui.Chip("cap", memoryCap, ui.Warn),
		ui.Chip("arch", machine.Arch, ui.Accent) + "  " + ui.Chip("cores", fmt.Sprint(machine.Cores), ui.Accent) + "  " + ui.Chip("network", onOff(network), ui.Good),
		ui.Chip("cache", cacheLabel, cacheColor) + "  " + ui.Chip("env/cache size", project.FormatBytes(status.CacheSize), ui.Accent),
		ui.Chip("configured services", fmt.Sprint(len(cfg.Up)), ui.Accent) + "  " + ui.Chip("running services", fmt.Sprint(serviceCount), ui.Good) + "  " + ui.Chip("shells", fmt.Sprint(shellCount), ui.Good),
		ui.Chip("project", project.ShortPath(status.ProjectPath), ui.Accent),
	}
	if !status.CacheReady {
		lines = append(lines, ui.Bullet(ui.Hot.Render("!"), "cold cache — first lagoon shell/up will resolve packages"))
	}
	if len(cfg.Up) > 0 {
		lines = append(lines, "", ui.Title.Render("Configured services"))
		for _, name := range sortedServiceNames(cfg.Up) {
			command := cfg.Up[name]
			lines = append(lines, fmt.Sprintf("  %s %-10s ports:%-10s %s", ui.Dim.Render("◌"), name, portsLabel(inferPorts(command)), ui.Dim.Render(command)))
		}
	}
	if len(entries) == 0 {
		scope := "this project"
		if allProjects {
			scope = "any project"
		}
		lines = append(lines, ui.Bullet(ui.Dim.Render("•"), "nothing running for "+scope))
	} else {
		lines = append(lines, "", ui.Title.Render("Running"))
		for _, e := range entries {
			name := e.Name
			if name == "" {
				name = "shell"
			}
			started, _ := time.Parse(time.RFC3339, e.Started)
			uptime := "?"
			if !started.IsZero() {
				uptime = time.Since(started).Round(time.Second).String()
			}
			mem := readProcessMem(e.PID)
			ports := e.Ports
			if len(ports) == 0 && e.Command != "" {
				ports = inferPorts(e.Command)
			}
			projectLabel := ""
			if allProjects {
				projectLabel = "  " + ui.Dim.Render(project.ShortPath(e.Project))
			}
			lines = append(lines, fmt.Sprintf("  %s %-10s pid:%-6d ports:%-10s mem:%-8s up:%s%s", ui.OK.Render("●"), name, e.PID, portsLabel(ports), mem, uptime, projectLabel))
			if e.Command != "" {
				lines = append(lines, "     "+ui.Dim.Render(e.Command))
			}
		}
	}
	return ui.Card("Lagoon ps", lines...)
}

func runningEntries(cacheBase, currentProject string, allProjects bool) []sandboxPID {
	files, _ := filepath.Glob(filepath.Join(cacheBase, "*", "*.pid.json"))
	legacy, _ := filepath.Glob(filepath.Join(cacheBase, "*", "pid.json"))
	files = append(files, legacy...)
	seen := map[string]bool{}
	var entries []sandboxPID
	for _, file := range files {
		if seen[file] {
			continue
		}
		seen[file] = true
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var info sandboxPID
		if err := json.Unmarshal(b, &info); err != nil || info.PID <= 0 {
			continue
		}
		if !isProcessAlive(info.PID) {
			continue
		}
		if !includeEntryForProject(info, currentProject, allProjects) {
			continue
		}
		if info.Kind == "" {
			info.Kind = "shell"
		}
		entries = append(entries, info)
	}
	return entries
}

// isProcessAlive sends signal 0 to check if a process exists.
func isProcessAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// readProcessMem returns the resident memory of a process. Linux reads VmRSS
// from /proc; on macOS the workload runs inside a VM the host can't meter per
// process, so we show "—" instead of a misleading number.
func readProcessMem(pid int) string {
	if runtime.GOOS != "linux" || pid <= 0 {
		return "—"
	}
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

func sortedServiceNames(services map[string]string) []string {
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func includeEntryForProject(info sandboxPID, currentProject string, allProjects bool) bool {
	return allProjects || info.Project == currentProject
}
