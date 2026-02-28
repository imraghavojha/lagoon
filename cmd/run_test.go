package cmd

import "testing"

func TestShellQuoteArgsSimple(t *testing.T) {
	got := shellQuoteArgs([]string{"ls", "-la"})
	if got != "'ls' '-la'" {
		t.Errorf("got %q", got)
	}
}

func TestShellQuoteArgsWithSpaces(t *testing.T) {
	// args with spaces must survive bash -c without being split
	got := shellQuoteArgs([]string{"ls", "my folder"})
	if got != "'ls' 'my folder'" {
		t.Errorf("got %q", got)
	}
}

func TestShellQuoteArgsSingleQuote(t *testing.T) {
	// single quotes inside args must be escaped via the '\'' technique
	got := shellQuoteArgs([]string{"echo", "it's"})
	if got != `'echo' 'it'\''s'` {
		t.Errorf("got %q", got)
	}
}

func TestShellQuoteArgsSingle(t *testing.T) {
	got := shellQuoteArgs([]string{"python3"})
	if got != "'python3'" {
		t.Errorf("got %q", got)
	}
}
