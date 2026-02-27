package nix

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kuldeepojha/lagoon/internal/config"
)

// ResolvedEnv holds the paths we capture from running nix-shell
type ResolvedEnv struct {
	BashPath string
	EnvPath  string
	PATH     string
}

// GenerateShellNix writes the shell.nix to outPath, creating parent dirs if needed
func GenerateShellNix(cfg *config.Config, outPath string) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	// indent each package name to match the nix template style
	var lines []string
	for _, p := range cfg.Packages {
		lines = append(lines, "    "+p)
	}

	content := shellNixTemplate
	content = strings.ReplaceAll(content, "{{COMMIT}}", cfg.NixpkgsCommit)
	content = strings.ReplaceAll(content, "{{SHA256}}", cfg.NixpkgsSHA256)
	content = strings.ReplaceAll(content, "{{PACKAGES}}", strings.Join(lines, "\n"))

	return os.WriteFile(outPath, []byte(content), 0644)
}

// missingAttrRe matches the nix error for unknown package names
var missingAttrRe = regexp.MustCompile(`attribute '([^']+)' missing`)

// Resolve runs nix-shell and grabs the bash path, env path, and PATH value.
// we run 'which bash && which env && echo $PATH' inside the nix shell to get them.
func Resolve(shellNixPath string) (*ResolvedEnv, error) {
	cmd := exec.Command("nix-shell", shellNixPath, "--run",
		"which bash && which env && echo $PATH")
	out, err := cmd.CombinedOutput()

	if err != nil {
		return nil, parseNixError(out)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("nix-shell output was unexpected:\n%s", string(out))
	}

	return &ResolvedEnv{
		BashPath: strings.TrimSpace(lines[0]),
		EnvPath:  strings.TrimSpace(lines[1]),
		PATH:     strings.TrimSpace(lines[len(lines)-1]), // last line is PATH, avoid any extra output
	}, nil
}

// parseNixError turns the raw nix error into something a human can act on
func parseNixError(output []byte) error {
	raw := string(output)

	// look for the common "attribute missing" pattern — that's a typo'd package name
	if m := missingAttrRe.FindStringSubmatch(raw); m != nil {
		return fmt.Errorf("✗ package not found: %s\n  search for the correct name at: https://search.nixos.org/packages\n  then update your lagoon.toml", m[1])
	}

	// unknown error — show it but label it clearly so users know what they're looking at
	return fmt.Errorf("nix-shell failed\n--- raw nix output ---\n%s", raw)
}
