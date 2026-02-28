package cmd

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWaitNixLineClosedChannel(t *testing.T) {
	ch := make(chan string)
	close(ch)
	msg := waitNixLine(ch)()
	if _, ok := msg.(nixDoneMsg); !ok {
		t.Errorf("expected nixDoneMsg on closed channel, got %T", msg)
	}
}

func TestWaitNixLineMessage(t *testing.T) {
	ch := make(chan string, 1)
	ch <- "building foo-1.0"
	msg := waitNixLine(ch)()
	line, ok := msg.(nixLineMsg)
	if !ok {
		t.Fatalf("expected nixLineMsg, got %T", msg)
	}
	if string(line) != "building foo-1.0" {
		t.Errorf("expected 'building foo-1.0', got %q", string(line))
	}
}

func TestBuildModelUpdateDone(t *testing.T) {
	ch := make(chan string)
	close(ch)
	m := newBuildModel(ch)
	_, cmd := m.Update(nixDoneMsg{})
	if cmd == nil {
		t.Fatal("expected a tea command from nixDoneMsg")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg from nixDoneMsg update")
	}
}

func TestBuildModelUpdateLine(t *testing.T) {
	ch := make(chan string, 1)
	m := newBuildModel(ch)
	newM, _ := m.Update(nixLineMsg("fetching openssl"))
	bm := newM.(buildModel)
	if bm.lastLine != "fetching openssl" {
		t.Errorf("lastLine not updated: got %q", bm.lastLine)
	}
}

func TestBuildModelViewTruncates(t *testing.T) {
	ch := make(chan string)
	close(ch)
	m := newBuildModel(ch)
	longLine := strings.Repeat("x", 80)
	newM, _ := m.Update(nixLineMsg(longLine))
	view := newM.(buildModel).View()
	// the view must not contain 61+ consecutive x chars
	if strings.Contains(view, strings.Repeat("x", 61)) {
		t.Error("view must truncate lines longer than 60 chars")
	}
	if !strings.Contains(view, "…") {
		t.Error("truncated view must contain ellipsis")
	}
}
