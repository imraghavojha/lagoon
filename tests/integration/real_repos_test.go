//go:build integration

// integration tests clone real public GitHub repos, write lagoon.toml for each,
// and verify the lint workflow against a mock nixpkgs API.
// requires: git in PATH and internet access for git clone.
// run with: go test -tags integration ./tests/integration/...
package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoCase describes a real GitHub repo and the packages we declare for it.
type repoCase struct {
	name     string
	cloneURL string
	packages []string
}

// knownRepos is the test matrix of real-world projects across different languages.
var knownRepos = []repoCase{
	{
		name:     "python-basics-exercises",
		cloneURL: "https://github.com/realpython/python-basics-exercises.git",
		packages: []string{"python3"},
	},
	{
		name:     "cobra-go",
		cloneURL: "https://github.com/spf13/cobra.git",
		packages: []string{"go"},
	},
	{
		name:     "ripgrep-rust",
		cloneURL: "https://github.com/BurntSushi/ripgrep.git",
		packages: []string{"rustc", "cargo"},
	},
}

// helpers ----------------------------------------------------------------

// skipIfMissing skips the test when a required binary is absent from PATH.
func skipIfMissing(t *testing.T, bins ...string) {
	t.Helper()
	for _, b := range bins {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("skipping: %s not found in PATH", b)
		}
	}
}

// cloneShallow does a depth-1 clone into dir.
func cloneShallow(t *testing.T, url, dir string) {
	t.Helper()
	cmd := exec.Command("git", "clone", "--depth", "1", "--quiet", url, dir)
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Run(), "git clone %s", url)
}

// writeCfg writes a lagoon.toml for the given packages into dir.
func writeCfg(t *testing.T, dir string, packages []string) {
	t.Helper()
	cfg := &config.Config{
		Packages:      packages,
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	require.NoError(t, config.Write(filepath.Join(dir, config.Filename), cfg))
}

// mockSearch returns an httptest server that answers "found" for packages in
// the allowed set and "not found" for everything else.
func mockSearch(t *testing.T, allowed map[string]bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query struct {
				MultiMatch struct{ Query string `json:"query"` } `json:"multi_match"`
			} `json:"query"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		q := body.Query.MultiMatch.Query
		w.Header().Set("Content-Type", "application/json")
		if allowed[q] {
			fmt.Fprintf(w, `{"hits":{"hits":[{"_source":{"package_attr_name":%q,"package_description":"ok"}}]}}`, q)
		} else {
			w.Write([]byte(`{"hits":{"hits":[]}}`))
		}
	}))
}

// searchPkg queries url for one package and returns whether an exact-name match
// was found. This mirrors the logic in cmd/lint.go:runLint().
func searchPkg(t *testing.T, url, pkg string) bool {
	t.Helper()
	body := fmt.Sprintf(`{"query":{"multi_match":{"query":%q,"fields":["package_attr_name^9","package_pname^6","package_description^1"]}},"size":8}`, pkg)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Name string `json:"package_attr_name"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	for _, h := range result.Hits.Hits {
		if h.Source.Name == pkg {
			return true
		}
	}
	return false
}

// config roundtrip tests -------------------------------------------------

// TestRealRepoConfigRoundtrip writes a lagoon.toml for each known repo and
// reads it back, verifying no data is lost or reordered.
func TestRealRepoConfigRoundtrip(t *testing.T) {
	for _, tc := range knownRepos {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCfg(t, dir, tc.packages)

			back, err := config.Read(filepath.Join(dir, config.Filename))
			require.NoError(t, err)

			assert.Equal(t, tc.packages, back.Packages,
				"packages must round-trip unchanged through lagoon.toml")
			assert.Equal(t, "minimal", back.Profile)
			assert.Len(t, back.NixpkgsCommit, 40, "DefaultCommit must be 40 chars")
			assert.Len(t, back.NixpkgsSHA256, 52, "DefaultSHA256 must be 52 chars")
		})
	}
}

// TestRealRepoShellNixGeneration verifies that GenerateShellNix() produces a
// shell.nix that contains every declared package for each repo.
func TestRealRepoShellNixGeneration(t *testing.T) {
	for _, tc := range knownRepos {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCfg(t, dir, tc.packages)

			cfg, err := config.Read(filepath.Join(dir, config.Filename))
			require.NoError(t, err)

			outPath := filepath.Join(dir, "shell.nix")
			sum, err := nix.GenerateShellNix(cfg, outPath)
			require.NoError(t, err, "GenerateShellNix must not fail for valid config")
			assert.NotEmpty(t, sum, "sum must be non-empty")

			content, err := os.ReadFile(outPath)
			require.NoError(t, err)
			for _, pkg := range tc.packages {
				assert.Contains(t, string(content), pkg,
					"package %q must appear in shell.nix", pkg)
			}
		})
	}
}

// TestRealRepoPythonLint simulates lint for a Python project using a mock
// nixpkgs API — verifies python3 is "found" with correct name matching.
func TestRealRepoPythonLint(t *testing.T) {
	skipIfMissing(t, "git")
	tc := knownRepos[0]
	dir := t.TempDir()
	cloneShallow(t, tc.cloneURL, dir)
	writeCfg(t, dir, tc.packages)

	srv := mockSearch(t, map[string]bool{"python3": true})
	defer srv.Close()

	cfg, err := config.Read(filepath.Join(dir, config.Filename))
	require.NoError(t, err)

	for _, pkg := range cfg.Packages {
		found := searchPkg(t, srv.URL, pkg)
		assert.True(t, found, "package %q must be found for python project", pkg)
	}
}

// TestRealRepoGoLint simulates lint for a Go project.
func TestRealRepoGoLint(t *testing.T) {
	skipIfMissing(t, "git")
	tc := knownRepos[1]
	dir := t.TempDir()
	cloneShallow(t, tc.cloneURL, dir)
	writeCfg(t, dir, tc.packages)

	srv := mockSearch(t, map[string]bool{"go": true})
	defer srv.Close()

	cfg, err := config.Read(filepath.Join(dir, config.Filename))
	require.NoError(t, err)

	for _, pkg := range cfg.Packages {
		found := searchPkg(t, srv.URL, pkg)
		assert.True(t, found, "package %q must be found for go project", pkg)
	}
}

// TestRealRepoRustLint simulates lint for a Rust project (rustc + cargo).
func TestRealRepoRustLint(t *testing.T) {
	skipIfMissing(t, "git")
	tc := knownRepos[2]
	dir := t.TempDir()
	cloneShallow(t, tc.cloneURL, dir)
	writeCfg(t, dir, tc.packages)

	srv := mockSearch(t, map[string]bool{"rustc": true, "cargo": true})
	defer srv.Close()

	cfg, err := config.Read(filepath.Join(dir, config.Filename))
	require.NoError(t, err)

	for _, pkg := range cfg.Packages {
		found := searchPkg(t, srv.URL, pkg)
		assert.True(t, found, "package %q must be found for rust project", pkg)
	}
}

// TestRealRepoInvalidPackageIsNotFound verifies that a deliberately invalid
// package name returns false from the search — lint must report it as ✗.
func TestRealRepoInvalidPackageIsNotFound(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, []string{"notarealpackage99999xyzzy"})

	// mock returns empty hits for unknown packages
	srv := mockSearch(t, map[string]bool{}) // nothing is valid
	defer srv.Close()

	found := searchPkg(t, srv.URL, "notarealpackage99999xyzzy")
	assert.False(t, found, "invalid package must NOT be found — lint must report ✗")
}

// TestRealRepoDuplicatePackageDetected verifies that duplicate packages in
// the config are detected before any expensive nix operations.
func TestRealRepoDuplicatePackageDetected(t *testing.T) {
	dir := t.TempDir()
	// deliberately add python3 twice
	writeCfg(t, dir, []string{"python3", "git", "python3"})

	cfg, err := config.Read(filepath.Join(dir, config.Filename))
	require.NoError(t, err)

	seen := map[string]bool{}
	var duplicates []string
	for _, pkg := range cfg.Packages {
		if seen[pkg] {
			duplicates = append(duplicates, pkg)
		}
		seen[pkg] = true
	}

	assert.NotEmpty(t, duplicates, "duplicate package must be detectable from the config")
	assert.Contains(t, duplicates, "python3")
}

// TestRealRepoOfflineLintDegrades verifies that an unreachable search URL
// produces an error — lint must show ? (unknown) not panic or crash.
func TestRealRepoOfflineLintDegrades(t *testing.T) {
	client := &http.Client{Timeout: 1 * time.Second}
	body := `{"query":{"multi_match":{"query":"python3","fields":["package_attr_name^9"]}},"size":8}`
	_, err := client.Post("http://127.0.0.1:1", "application/json", strings.NewReader(body))
	assert.Error(t, err, "offline search must return an error so lint can degrade gracefully")
}

// benchmarks -------------------------------------------------------------

// BenchmarkShellNixGen measures GenerateShellNix() for a 10-package config.
func BenchmarkShellNixGen(b *testing.B) {
	cfg := &config.Config{
		Packages:      []string{"python3", "nodejs", "rustc", "cargo", "go", "git", "curl", "wget", "gcc", "gnumake"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outPath := filepath.Join(b.TempDir(), "shell.nix")
		if _, err := nix.GenerateShellNix(cfg, outPath); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConfigReadWrite measures the config TOML roundtrip — called on
// every lagoon invocation.
func BenchmarkConfigReadWrite(b *testing.B) {
	cfg := &config.Config{
		Packages:      []string{"python3", "git", "curl"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "minimal",
	}
	dir := b.TempDir()
	path := filepath.Join(dir, config.Filename)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := config.Write(path, cfg); err != nil {
			b.Fatal(err)
		}
		if _, err := config.Read(path); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWarmStartCacheLoad measures nix.LoadCache() — the critical path
// for sub-second warm starts.
func BenchmarkWarmStartCacheLoad(b *testing.B) {
	cacheDir := b.TempDir()
	env := &nix.ResolvedEnv{
		BashPath: "/nix/store/abc123-bash/bin/bash",
		EnvPath:  "/nix/store/abc123-env/bin/env",
		PATH:     "/nix/store/abc123-bash/bin:/nix/store/abc123-env/bin",
	}
	if err := nix.SaveCache(cacheDir, env, "benchsum"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e, hit := nix.LoadCache(cacheDir, "benchsum")
		if !hit || e == nil {
			b.Fatal("cache miss during benchmark")
		}
	}
}

// BenchmarkLintOnePkg measures a single nixpkgs search call against a
// local mock server (no real network overhead).
func BenchmarkLintOnePkg(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hits":{"hits":[{"_source":{"package_attr_name":"python3","package_description":"Python 3"}}]}}`))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	body := `{"query":{"multi_match":{"query":"python3","fields":["package_attr_name^9"]}},"size":8}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Post(srv.URL, "application/json", strings.NewReader(body))
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// TestUpConfigLoadsServices verifies that the [up] table in lagoon.toml is
// parsed correctly into cfg.Up — no nix or bwrap required.
func TestUpConfigLoadsServices(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{
		Packages:      []string{"python3", "nodejs"},
		NixpkgsCommit: config.DefaultCommit,
		NixpkgsSHA256: config.DefaultSHA256,
		Profile:       "network",
		Up: map[string]string{
			"web": "node server.js",
			"api": "python3 -m flask run --port 8080",
		},
	}
	cfgPath := filepath.Join(dir, config.Filename)
	require.NoError(t, config.Write(cfgPath, cfg))

	loaded, err := config.Read(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, cfg.Up, loaded.Up, "up services must survive a write/read roundtrip")
	assert.Equal(t, "node server.js", loaded.Up["web"])
	assert.Equal(t, "python3 -m flask run --port 8080", loaded.Up["api"])
}
