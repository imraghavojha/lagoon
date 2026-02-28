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
// matching stderr lines are sent to progress as they arrive; caller closes the channel after use.
func Resolve(shellNixPath string, progress chan<- string) (*ResolvedEnv, error) {
	var stderrBuf bytes.Buffer
	pr, pw := io.Pipe()
	cmd := exec.Command("nix-shell", shellNixPath, "--run",
		"which bash && which env && echo $PATH")
	cmd.Stderr = io.MultiWriter(&stderrBuf, pw)

	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			lower := strings.ToLower(line)
			for _, kw := range nixKeywords {
				if strings.Contains(lower, kw) {
					select {
					case progress <- line:
					default:
					}
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

	return parseResolveOutput(string(stdout))
}

// parseResolveOutput parses stdout from: which bash && which env && echo $PATH
// finds bash/env by suffix match rather than position so stray lines don't break it.
func parseResolveOutput(stdout string) (*ResolvedEnv, error) {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("nix-shell output was unexpected:\n%s", stdout)
	}

	// PATH is always the last line
	rawPath := strings.TrimSpace(lines[len(lines)-1])

	// only keep nix store paths — host paths like /usr/bin leak state into the sandbox
	var nixParts []string
	for _, p := range strings.Split(rawPath, ":") {
		if strings.HasPrefix(p, "/nix/store") {
			nixParts = append(nixParts, p)
		}
	}
	if len(nixParts) == 0 {
		return nil, fmt.Errorf("no nix store paths found in PATH — nix-shell may have failed silently")
	}

	// find bash and env by suffix — position-based parsing breaks if nix prints extra lines
	var bash, env string
	for _, l := range lines[:len(lines)-1] {
		l = strings.TrimSpace(l)
		if bash == "" && strings.HasSuffix(l, "/bash") {
			bash = l
		} else if env == "" && strings.HasSuffix(l, "/env") {
			env = l
		}
	}
	if bash == "" || env == "" {
		return nil, fmt.Errorf("could not find bash/env in nix-shell output:\n%s", stdout)
	}

	return &ResolvedEnv{BashPath: bash, EnvPath: env, PATH: strings.Join(nixParts, ":")}, nil
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
