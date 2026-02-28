package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAndReadPIDFile(t *testing.T) {
	dir := t.TempDir()
	writePIDFile(dir, "/home/user/myproject", []string{"python311", "ffmpeg"})

	b, err := os.ReadFile(filepath.Join(dir, "pid.json"))
	if err != nil {
		t.Fatalf("pid.json not written: %v", err)
	}

	var info sandboxPID
	if err := json.Unmarshal(b, &info); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if info.PID != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), info.PID)
	}
	if info.Project != "/home/user/myproject" {
		t.Errorf("wrong project: %s", info.Project)
	}
	if len(info.Packages) != 2 || info.Packages[0] != "python311" {
		t.Errorf("wrong packages: %v", info.Packages)
	}
	if info.Started == "" {
		t.Error("started timestamp must be set")
	}
}

func TestIsProcessAliveCurrentPID(t *testing.T) {
	// current process must always be alive
	if !isProcessAlive(os.Getpid()) {
		t.Error("current process should be alive")
	}
}

func TestIsProcessAliveDeadPID(t *testing.T) {
	// an extremely high pid that almost certainly doesn't exist
	if isProcessAlive(9999999) {
		t.Skip("pid 9999999 happens to exist — skipping")
	}
}

func TestReadProcessMem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("readProcessMem is linux-only")
	}
	// reading our own process memory should return a non-? result
	mem := readProcessMem(os.Getpid())
	if mem == "?" {
		t.Error("expected valid memory reading for current process")
	}
}

func TestReadProcessMemFakeProc(t *testing.T) {
	// non-existent pid returns "?"
	result := readProcessMem(9999999)
	if result != "?" {
		t.Errorf("expected '?' for dead pid, got %q", result)
	}
}
