package sandbox

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func TestEnterMemoryRequiresSystemdRun(t *testing.T) {
	// create a fake bwrap so LookPath("bwrap") succeeds
	dir := t.TempDir()
	bwrapFake := filepath.Join(dir, "bwrap")
	if err := os.WriteFile(bwrapFake, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	// PATH only contains the temp dir — no systemd-run there
	t.Setenv("PATH", dir)

	err := Enter(fakeCfg("minimal"), fakeEnv(), "/tmp", "", "512m", nil)
	if err == nil {
		t.Fatal("expected error when systemd-run is not found")
	}
	if !strings.Contains(err.Error(), "systemd-run") {
		t.Errorf("error should mention systemd-run, got: %v", err)
	}
}

func TestEnterNoBwrap(t *testing.T) {
	// empty PATH — nothing is findable
	t.Setenv("PATH", t.TempDir())

	err := Enter(fakeCfg("minimal"), fakeEnv(), "/tmp", "", "", nil)
	if err == nil {
		t.Fatal("expected error when bwrap is not found")
	}
	if !strings.Contains(err.Error(), "bwrap") {
		t.Errorf("error should mention bwrap, got: %v", err)
	}
}

// on_enter hook tests

func fakeCfgWithHook(profile, hook string) *config.Config {
	return &config.Config{Profile: profile, Packages: []string{"cowsay"}, OnEnter: hook}
}

func TestBuildArgsOnEnterInteractive(t *testing.T) {
	env := fakeEnv()
	args := buildArgs(fakeCfgWithHook("minimal", "source .env"), env, "/proj", "", nil)
	// must end with: -- bash -c "source .env; exec bash"
	n := len(args)
	if n < 4 {
		t.Fatalf("too few args: %v", args)
	}
	if args[n-4] != "--" || args[n-3] != env.BashPath || args[n-2] != "-c" {
		t.Errorf("expected [-- bash -c ...], got %v", args[n-4:])
	}
	if !strings.Contains(args[n-1], "source .env") {
		t.Errorf("on_enter hook missing from arg: %q", args[n-1])
	}
	if !strings.Contains(args[n-1], "exec "+env.BashPath) {
		t.Errorf("exec bash missing from hook arg: %q", args[n-1])
	}
}

func TestBuildArgsOnEnterWithCommand(t *testing.T) {
	env := fakeEnv()
	args := buildArgs(fakeCfgWithHook("minimal", "source .env"), env, "/proj", "cowsay hi", nil)
	n := len(args)
	if args[n-2] != "-c" {
		t.Errorf("expected -c flag, got %v", args[n-2:])
	}
	if !strings.Contains(args[n-1], "source .env") {
		t.Errorf("on_enter missing: %q", args[n-1])
	}
	if !strings.Contains(args[n-1], "cowsay hi") {
		t.Errorf("command missing: %q", args[n-1])
	}
}

func TestBuildArgsNoHookNoChange(t *testing.T) {
	env := fakeEnv()
	args := buildArgs(fakeCfg("minimal"), env, "/proj", "", nil)
	n := len(args)
	// no hook → ends with [-- bash] (no -c)
	if args[n-2] != "--" || args[n-1] != env.BashPath {
		t.Errorf("without hook, expected [-- bash], got %v", args[n-2:])
	}
}

func TestStartNoBwrap(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Start(fakeCfg("minimal"), fakeEnv(), "/tmp", "echo hi", "", nil)
	if err == nil {
		t.Fatal("expected error when bwrap not found")
	}
	if !strings.Contains(err.Error(), "bwrap") {
		t.Errorf("error should mention bwrap, got: %v", err)
	}
}
