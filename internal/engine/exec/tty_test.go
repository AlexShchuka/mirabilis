//go:build darwin || linux

package exec

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestTTYRunsChildWithProvidedStdio(t *testing.T) {
	var out syncBuffer
	cmd := &TTY{Argv: []string{"/bin/sh", "-c", "printf hello; printf world >&2"}}
	cmd.SetStdin(strings.NewReader(""))
	cmd.SetStdout(&out)
	var errBuf syncBuffer
	cmd.SetStderr(&errBuf)
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out.String() != "hello" || errBuf.String() != "world" {
		t.Fatalf("stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestTTYExitError(t *testing.T) {
	cmd := &TTY{Argv: []string{"/bin/sh", "-c", "exit 3"}}
	cmd.SetStdout(&syncBuffer{})
	cmd.SetStderr(&syncBuffer{})
	if err := cmd.Run(); err == nil {
		t.Fatal("expected exit error")
	}
}

func TestPTYTeeChildSeesTTYAndTeeCaptures(t *testing.T) {
	var tee syncBuffer
	var out syncBuffer
	cmd := NewPTYTee([]string{"/bin/sh", "-c", "test -t 0 && test -t 1 && echo sk-ant-oat01-fromtty || echo notty"}, &tee)
	cmd.SetStdout(&out)
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("pty run timed out")
	}
	if !strings.Contains(tee.String(), "sk-ant-oat01-fromtty") {
		t.Fatalf("tee missed output: %q", tee.String())
	}
	if !strings.Contains(out.String(), "sk-ant-oat01-fromtty") {
		t.Fatalf("stdout missed output: %q", out.String())
	}
	if strings.Contains(out.String(), "notty") {
		t.Fatal("child did not get a tty")
	}
}

func TestPTYTeeStdinReachesChild(t *testing.T) {
	var out syncBuffer
	cmd := NewPTYTee([]string{"/bin/sh", "-c", "read line; printf '%s' \"$line\""}, nil)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	go func() {
		_, _ = w.WriteString("ping\n")
		w.Close()
	}()
	cmd.SetStdin(r)
	cmd.SetStdout(&out)
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("pty stdin run timed out")
	}
	if !strings.Contains(out.String(), "ping") {
		t.Fatalf("child did not receive stdin: %q", out.String())
	}
}
