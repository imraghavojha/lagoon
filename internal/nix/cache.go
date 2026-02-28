package nix

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const cacheFile = "env.json"

// cachedEnv is what we persist to disk after a successful nix-shell resolve
type cachedEnv struct {
	Sum      string `json:"sum"` // sha256 of shell.nix content — cache is invalid if this changes
	BashPath string `json:"bash_path"`
	EnvPath  string `json:"env_path"`
	PATH     string `json:"path"`
}

// contentSum returns a short hex hash of b
func contentSum(b []byte) string {
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

// LoadCache returns a cached ResolvedEnv if the sum matches (i.e. shell.nix unchanged).
// returns nil, false on any miss or error.
func LoadCache(cacheDir, sum string) (*ResolvedEnv, bool) {
	data, err := os.ReadFile(filepath.Join(cacheDir, cacheFile))
	if err != nil {
		return nil, false
	}
	var c cachedEnv
	if json.Unmarshal(data, &c) != nil || c.Sum != sum {
		return nil, false
	}
	return &ResolvedEnv{BashPath: c.BashPath, EnvPath: c.EnvPath, PATH: c.PATH}, true
}

// SaveCache writes the resolved env to disk. errors here are non-fatal.
func SaveCache(cacheDir string, env *ResolvedEnv, sum string) error {
	c := cachedEnv{Sum: sum, BashPath: env.BashPath, EnvPath: env.EnvPath, PATH: env.PATH}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cacheDir, cacheFile), data, 0644)
}
