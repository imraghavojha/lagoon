package nix

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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

// nixKeywords are the substrings we surface from nix stderr so users see progress
var nixKeywords = []string{"fetching", "downloading", "building", "copying", "error", "warning"}

// Resolve runs nix-shell and grabs the bash path, env path, and PATH value.
// nix stderr is filtered and streamed so users see meaningful progress lines
// instead of a frozen terminal during long first-run builds.
func Resolve(shellNixPath string) (*ResolvedEnv, error) {
	var stderrBuf bytes.Buffer
	pr, pw := io.Pipe()
	cmd := exec.Command("nix-shell", shellNixPath, "--run",
		"which bash && which env && echo $PATH")
	cmd.Stderr = io.MultiWriter(&stderrBuf, pw)

	// stream matching nix stderr lines dimmed to stdout
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			lower := strings.ToLower(line)
			for _, kw := range nixKeywords {
				if strings.Contains(lower, kw) {
					fmt.Printf("\033[2m  nix │ %s\033[0m\n", line)
					break
				}
			}
		}
	}()

	stdout, err := cmd.Output()
	pw.Close()
	<-done

	if err != nil {
		return nil, parseNixError(stderrBuf.Bytes())
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
