package project

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
)

// Status is a read-only snapshot of the current Lagoon project.
type Status struct {
	ProjectPath string
	CacheDir    string
	ShellSum    string
	CacheReady  bool
	CacheCold   bool
	CacheSize   int64
	EnvMissing  bool
}

func Inspect(cfg *config.Config, projectPath, cacheBase string) Status {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		abs = projectPath
	}
	cacheDir := ProjectCacheDir(cacheBase, abs)
	_, sum := nix.RenderShellNix(cfg)
	resolved, ready := nix.LoadCache(cacheDir, sum)
	envMissing := false
	if ready {
		if _, err := os.Stat(resolved.BashPath); err != nil {
			ready = false
			envMissing = true
		}
	}
	return Status{
		ProjectPath: abs,
		CacheDir:    cacheDir,
		ShellSum:    sum,
		CacheReady:  ready,
		CacheCold:   !ready,
		CacheSize:   DirSize(cacheDir),
		EnvMissing:  envMissing,
	}
}

func ProjectCacheDir(cacheBase, absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	return filepath.Join(cacheBase, fmt.Sprintf("%x", h[:4]))
}

func DirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func FormatBytes(v int64) string {
	if v <= 0 {
		return "0 B"
	}
	units := []string{"B", "KiB", "MiB", "GiB"}
	f := float64(v)
	unit := 0
	for f >= 1024 && unit < len(units)-1 {
		f /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", v, units[unit])
	}
	return fmt.Sprintf("%.1f %s", f, units[unit])
}

func CacheLabel(ready bool) string {
	if ready {
		return "warm"
	}
	return "cold"
}

func ShortPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
