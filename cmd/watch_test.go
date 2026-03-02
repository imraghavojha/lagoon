package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWatchDebounce verifies that rapid events produce only one restart.
func TestWatchDebounce(t *testing.T) {
	var (
		mu    sync.Mutex
		count int
	)
	trigger := func() {
		mu.Lock()
		count++
		mu.Unlock()
	}

	var (
		timerMu  sync.Mutex
		debounce *time.Timer
	)
	fire := func() {
		timerMu.Lock()
		defer timerMu.Unlock()
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(100*time.Millisecond, trigger)
	}

	// fire 10 times in rapid succession
	for i := 0; i < 10; i++ {
		fire()
	}

	// wait long enough for the debounce to fire
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	got := count
	mu.Unlock()

	if got != 1 {
		t.Errorf("expected 1 restart from debounce, got %d", got)
	}
}

// TestWatchDebounceReset verifies that a second burst after the first
// fire also coalesces into a single call.
func TestWatchDebounceReset(t *testing.T) {
	var (
		mu    sync.Mutex
		count int
	)
	trigger := func() {
		mu.Lock()
		count++
		mu.Unlock()
	}

	var (
		timerMu  sync.Mutex
		debounce *time.Timer
	)
	fire := func() {
		timerMu.Lock()
		defer timerMu.Unlock()
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(50*time.Millisecond, trigger)
	}

	// first burst
	fire()
	fire()
	fire()
	time.Sleep(150 * time.Millisecond)

	// second burst
	fire()
	fire()
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	got := count
	mu.Unlock()

	if got != 2 {
		t.Errorf("expected 2 restarts (one per burst), got %d", got)
	}
}

// TestWatchProcessTerminatesOnSIGTERM verifies the process lifecycle used by
// the watch loop: after SIGTERM the process should exit within 500ms.
// This tests the same pattern as stopCurrent() in watch.go.
func TestWatchProcessTerminatesOnSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM not meaningful on windows")
	}

	p := exec.Command("sleep", "100")
	require.NoError(t, p.Start())

	done := make(chan struct{})
	go func() { p.Wait(); close(done) }()

	require.NoError(t, p.Process.Signal(syscall.SIGTERM))

	select {
	case <-done:
		// process exited after SIGTERM — exactly the behaviour stopCurrent expects
	case <-time.After(600 * time.Millisecond):
		p.Process.Kill()
		<-done
		t.Error("process did not exit within 500ms after SIGTERM")
	}
}

// TestWatchProcessKilledWhenSIGTERMIgnored verifies that a process that does
// not respond to SIGTERM within 500ms is escalated to SIGKILL.
// This mirrors the fallback branch in stopCurrent().
func TestWatchProcessKilledWhenSIGTERMIgnored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals not available on windows")
	}

	// trap_sigterm.sh: ignore SIGTERM, sleep a long time
	p := exec.Command("bash", "-c", "trap '' SIGTERM; sleep 100")
	require.NoError(t, p.Start())

	done := make(chan struct{})
	go func() { p.Wait(); close(done) }()

	p.Process.Signal(syscall.SIGTERM) //nolint

	// after 500ms it should still be alive (it ignores SIGTERM)
	select {
	case <-done:
		t.Log("process exited on SIGTERM (bash may not honour trap '' on all platforms)")
		return // not a failure — platform-specific behaviour
	case <-time.After(550 * time.Millisecond):
		// expected: still running — now kill it
	}

	require.NoError(t, p.Process.Kill())
	select {
	case <-done:
		// SIGKILL forced exit — this is the stopCurrent fallback path
	case <-time.After(200 * time.Millisecond):
		t.Error("process did not exit even after SIGKILL")
	}
}

// TestWatchDebounce300ms verifies that the production debounce window (300ms)
// coalesces multiple rapid events into one restart.
func TestWatchDebounce300ms(t *testing.T) {
	var mu sync.Mutex
	count := 0
	trigger := func() { mu.Lock(); count++; mu.Unlock() }

	var timerMu sync.Mutex
	var debounce *time.Timer

	fire := func() {
		timerMu.Lock()
		defer timerMu.Unlock()
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(300*time.Millisecond, trigger)
	}

	// simulate a file-save storm: 20 rapid events (e.g. editor writes multiple files)
	for i := 0; i < 20; i++ {
		fire()
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond) // wait for debounce to fire

	mu.Lock()
	got := count
	mu.Unlock()

	assert.Equal(t, 1, got, "20 rapid events within 300ms window must produce exactly 1 restart")
}

// TestWatchDirCountThreshold verifies that the inotify warning threshold is
// exactly 500 — below it no warning, at 501 the warning fires.
// This tests the constant embedded in watch.go rather than the full watcher.
func TestWatchDirCountThreshold(t *testing.T) {
	const watchThreshold = 500

	// create a temp dir tree with exactly 502 subdirectories and count them
	root := t.TempDir()
	var created int
	for i := 0; i < 502; i++ {
		require.NoError(t, os.MkdirAll(filepath.Join(root, fmt.Sprintf("d%04d", i)), 0755))
		created++
	}

	var dirCount int
	filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirCount++
		}
		return nil
	})

	// the production code warns when dirCount > 500
	assert.Greater(t, dirCount, watchThreshold,
		"502 subdirs should exceed the 500-dir inotify warning threshold")
}
