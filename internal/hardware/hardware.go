package hardware

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// Class is Lagoon's intentionally small hardware vocabulary. It guides
// defaults without blocking users from overriding them.
type Class string

const (
	PiClass     Class = "Pi-class"
	LaptopClass Class = "Laptop-class"
	MiniPCClass Class = "Mini-PC"
)

// Machine describes the local host in the terms the CLI needs for guidance.
type Machine struct {
	TotalRAMMiB int64
	Cores       int
	Arch        string
	DiskFreeMiB int64
	Class       Class
}

// Detect reads best-effort local machine facts. Missing facts are left at zero;
// callers should present them as unknown rather than failing init/status flows.
func Detect(path string) Machine {
	if path == "" {
		path = "."
	}
	m := Machine{
		TotalRAMMiB: readLinuxMemTotalMiB(),
		Cores:       runtime.NumCPU(),
		Arch:        runtime.GOARCH,
		DiskFreeMiB: diskFreeMiB(path),
	}
	m.Class = Classify(m.TotalRAMMiB, m.Cores, m.Arch)
	return m
}

// Classify assigns a human-friendly machine class. The thresholds are
// deliberately conservative and only affect recommendations.
func Classify(ramMiB int64, cores int, arch string) Class {
	arch = strings.ToLower(arch)
	if ramMiB > 0 && ramMiB <= 4096 {
		return PiClass
	}
	if strings.Contains(arch, "arm") && cores <= 4 {
		return PiClass
	}
	if ramMiB >= 16384 || cores >= 8 {
		return MiniPCClass
	}
	return LaptopClass
}

// DefaultMemoryCap returns a practical cap for local shells/services. Empty
// means no default cap because RAM could not be detected.
func DefaultMemoryCap(m Machine) string {
	switch m.Class {
	case PiClass:
		return "768m"
	case MiniPCClass:
		return "4g"
	default:
		return "2g"
	}
}

// UpRecommended tells the CLI whether service stacks are likely comfortable by
// default. Users can still run up on any machine.
func UpRecommended(m Machine) bool {
	return m.TotalRAMMiB == 0 || m.TotalRAMMiB >= 2048
}

// Warnings returns first-run caveats based on detected hardware.
func Warnings(m Machine) []string {
	var warnings []string
	if strings.Contains(strings.ToLower(m.Arch), "arm") {
		warnings = append(warnings, "ARM cold starts can compile packages; warm starts use the cache.")
	}
	if m.TotalRAMMiB > 0 && m.TotalRAMMiB < 2048 {
		warnings = append(warnings, "Less than 2 GiB RAM detected; prefer shells and one small service.")
	}
	if m.DiskFreeMiB > 0 && m.DiskFreeMiB < 4096 {
		warnings = append(warnings, "Low disk space; Nix closures and portable archives may fail.")
	}
	return warnings
}

func (m Machine) Summary() string {
	return fmt.Sprintf("%s • %s • %d cores • %s RAM • %s free", m.Class, m.Arch, m.Cores, FormatMiB(m.TotalRAMMiB), FormatMiB(m.DiskFreeMiB))
}

func FormatMiB(v int64) string {
	if v <= 0 {
		return "unknown"
	}
	if v >= 1024 {
		return fmt.Sprintf("%.1f GiB", float64(v)/1024)
	}
	return fmt.Sprintf("%d MiB", v)
}

func readLinuxMemTotalMiB() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kb / 1024
	}
	return 0
}

func diskFreeMiB(path string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize) / 1024 / 1024
}
