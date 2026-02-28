package cmd

import (
	"sync"
	"testing"
	"time"
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
