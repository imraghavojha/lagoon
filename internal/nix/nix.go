package nix

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/imraghavojha/lagoon/internal/config"
)

// ResolvedEnv holds the paths we capture from running nix-shell
type ResolvedEnv struct {
	BashPath string
	EnvPath  string
	PATH     string
}

// GenerateShellNix writes shell.nix to outPath if the content changed.
// returns the sha256 sum of the generated content for cache lookups.
func GenerateShellNix(cfg *config.Config, outPath string) (string, error) {
	var lines []string
	for _, p := range cfg.Packages {
		lines = append(lines, "    "+p)
	}

	content := shellNixTemplate
	content = strings.ReplaceAll(content, "{{COMMIT}}", cfg.NixpkgsCommit)
	content = strings.ReplaceAll(content, "{{SHA256}}", cfg.NixpkgsSHA256)
	content = strings.ReplaceAll(content, "{{PACKAGES}}", strings.Join(lines, "\n"))

	sum := contentSum([]byte(content))

	// skip the write if the file already has this exact content
	if existing, err := os.ReadFile(outPath); err == nil && contentSum(existing) == sum {
		return sum, nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", err
	}
	return sum, os.WriteFile(outPath, []byte(content), 0644)
}

// missingAttrRe matches the nix error for unknown package names
var missingAttrRe = regexp.MustCompile(`attribute '([^']+)' missing`)

// Resolve runs nix-shell and grabs the bash path, env path, and PATH value.
// we run 'which bash && which env && echo $PATH' inside the nix shell to get them.
// nix's build status messages go to stderr — we only parse stdout so they don't mix in.
func Resolve(shellNixPath string) (*ResolvedEnv, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("nix-shell", shellNixPath, "--run",
		"which bash && which env && echo $PATH")
	cmd.Stderr = &stderr

	stdout, err := cmd.Output()
	if err != nil {
		return nil, parseNixError(stderr.Bytes())
	}

	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("nix-shell output was unexpected:\n%s", string(stdout))
	}

	rawPath := strings.TrimSpace(lines[len(lines)-1])

	// only keep nix store paths — host paths like /usr/bin leak state into the sandbox
	var nixParts []string
	for _, p := range strings.Split(rawPath, ":") {
		if strings.HasPrefix(p, "/nix/store") {
			nixParts = append(nixParts, p)
		}
	}

	return &ResolvedEnv{
		BashPath: strings.TrimSpace(lines[0]),
		EnvPath:  strings.TrimSpace(lines[1]),
		PATH:     strings.Join(nixParts, ":"),
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
