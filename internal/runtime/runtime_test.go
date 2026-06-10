package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
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
}

func TestWithStderr_WrapsExitErrorStderr(t *testing.T) {
	r := NewLocalRunner()
	_, err := r.Host(context.Background(), "sh", "-c", "echo boom >&2; exit 7")
	if err == nil {
		t.Fatal("Host with failing command returned nil error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q does not contain stderr 'boom'", err.Error())
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Errorf("wrapped error %q no longer satisfies errors.As(*exec.ExitError)", err.Error())
	}
}

func TestWithStderr_PassesThroughNonExitError(t *testing.T) {
	if got := withStderr(nil); got != nil {
		t.Errorf("withStderr(nil) = %v, want nil", got)
	}
	plain := fmt.Errorf("plain error")
	if got := withStderr(plain); got != plain {
		t.Errorf("withStderr(plain) = %v, want the same error unchanged", got)
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
