package sandbox

import (
	"slices"
	"testing"

	"github.com/imraghavojha/lagoon/internal/config"
	"github.com/imraghavojha/lagoon/internal/nix"
)

func fakeEnv() *nix.ResolvedEnv {
	return &nix.ResolvedEnv{
		BashPath: "/nix/store/abc/bin/bash",
		EnvPath:  "/nix/store/def/bin/env",
		PATH:     "/nix/store/abc/bin:/nix/store/def/bin",
	}
}

func fakeCfg(profile string) *config.Config {
	return &config.Config{Profile: profile, Packages: []string{"cowsay"}}
}

// hasSeq reports whether needle appears contiguously in haystack.
func hasSeq(haystack []string, needle ...string) bool {
	for i := range haystack {
		if i+len(needle) > len(haystack) {
			break
		}
		if slices.Equal(haystack[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

func TestBuildArgsNixStoreMount(t *testing.T) {
	args := buildArgs(fakeCfg("minimal"), fakeEnv(), "/proj", "", nil)
	if !hasSeq(args, "--ro-bind", "/nix/store", "/nix/store") {
		t.Error("missing nix store ro-bind")
	}
}

func TestBuildArgsWorkspaceMount(t *testing.T) {
	args := buildArgs(fakeCfg("minimal"), fakeEnv(), "/my/project", "", nil)
	if !hasSeq(args, "--bind", "/my/project", "/workspace") {
		t.Error("missing workspace bind")
	}
}

func TestBuildArgsClearenv(t *testing.T) {
	args := buildArgs(fakeCfg("minimal"), fakeEnv(), "/proj", "", nil)
	if !slices.Contains(args, "--clearenv") {
		t.Error("missing --clearenv")
	}
}

func TestBuildArgsPS1(t *testing.T) {
	args := buildArgs(fakeCfg("minimal"), fakeEnv(), "/proj", "", nil)
	if !hasSeq(args, "--setenv", "PS1", "[lagoon] \\w $ ") {
		t.Error("missing PS1 setenv")
	}
}

func TestBuildArgsSymlinks(t *testing.T) {
	env := fakeEnv()
	args := buildArgs(fakeCfg("minimal"), env, "/proj", "", nil)
	if !hasSeq(args, "--symlink", env.BashPath, "/bin/bash") {
		t.Error("missing /bin/bash symlink")
	}
	if !hasSeq(args, "--symlink", env.EnvPath, "/usr/bin/env") {
		t.Error("missing /usr/bin/env symlink")
	}
}

func TestBuildArgsNetworkOff(t *testing.T) {
	args := buildArgs(fakeCfg("minimal"), fakeEnv(), "/proj", "", nil)
	if slices.Contains(args, "--share-net") {
		t.Error("--share-net must not appear for minimal profile")
	}
}

func TestBuildArgsNetworkOn(t *testing.T) {
	args := buildArgs(fakeCfg("network"), fakeEnv(), "/proj", "", nil)
	if !slices.Contains(args, "--share-net") {
		t.Error("--share-net must appear for network profile")
	}
}

func TestBuildArgsInteractiveShell(t *testing.T) {
	env := fakeEnv()
	args := buildArgs(fakeCfg("minimal"), env, "/proj", "", nil)
	// must end with: -- bashPath (no -c)
	n := len(args)
	if n < 2 || args[n-2] != "--" || args[n-1] != env.BashPath {
		t.Errorf("interactive shell: expected last args to be [-- %s], got %v", env.BashPath, args[n-2:])
	}
}

func TestBuildArgsOneOffCommand(t *testing.T) {
	env := fakeEnv()
	args := buildArgs(fakeCfg("minimal"), env, "/proj", "echo hi", nil)
	if !hasSeq(args, "--", env.BashPath, "-c", "echo hi") {
		t.Errorf("one-off command: expected [-- bash -c 'echo hi'], got tail: %v", args[len(args)-4:])
	}
}

func TestBuildArgsEnvInjection(t *testing.T) {
	args := buildArgs(fakeCfg("minimal"), fakeEnv(), "/proj", "", []string{"FOO=bar", "X=1"})
	if !hasSeq(args, "--setenv", "FOO", "bar") {
		t.Error("expected FOO=bar injection")
	}
	if !hasSeq(args, "--setenv", "X", "1") {
		t.Error("expected X=1 injection")
	}
}

func TestBuildArgsBadEnvSkipped(t *testing.T) {
	// entry with no '=' should not emit a --setenv
	args := buildArgs(fakeCfg("minimal"), fakeEnv(), "/proj", "", []string{"NOEQUALSSIGN"})
	for i, a := range args {
		if a == "--setenv" && i+1 < len(args) && args[i+1] == "NOEQUALSSIGN" {
			t.Error("bad env entry must not produce a --setenv")
		}
	}
}
