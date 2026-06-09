package runtime

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestResetTerminal(t *testing.T) {
	var buf bytes.Buffer
	resetTerminal(&buf)
	want := exitAltScreen + resetScrollRegion + clearScreenHome + showCursor
	got := buf.String()
	if got == "" {
		t.Fatal("resetTerminal wrote nothing")
	}
	if got != want {
		t.Fatalf("resetTerminal output mismatch: got %q, want %q", got, want)
	}
	if idx := strings.Index(got, exitAltScreen); idx < 0 {
		t.Fatal("exitAltScreen not found in output")
	}
	if idx := strings.Index(got, clearScreenHome); idx < 0 {
		t.Fatal("clearScreenHome not found in output")
	}
	altIdx := strings.Index(got, exitAltScreen)
	clearIdx := strings.Index(got, clearScreenHome)
	if altIdx > clearIdx {
		t.Error("exitAltScreen must precede clearScreenHome")
	}
}

func TestResolveGHTokenEmpty(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", nil
		},
	}
	_, err := resolveGHToken(r)
	if err == nil {
		t.Error("resolveGHToken must return an error when token is empty")
	}
}

func TestResolveGHTokenPresent(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "ghp_testtoken", nil
		},
	}
	tok, err := resolveGHToken(r)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if tok != "ghp_testtoken" {
		t.Errorf("got %q, want ghp_testtoken", tok)
	}
}
