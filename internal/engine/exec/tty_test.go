//go:build darwin || linux

package exec

import (
	"bytes"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const ptyTestDeadline = 5 * time.Second

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

func TestTTYChildInheritsForegroundPgroup(t *testing.T) {
	parentPgid, err := syscall.Getpgid(0)
	if err != nil {
		t.Fatalf("getpgid(self): %v", err)
	}
	var out syncBuffer
	cmd := &TTY{Argv: []string{"/bin/sh", "-c", "ps -o pgid= -p $$ | tr -d ' '"}}
	cmd.SetStdin(strings.NewReader(""))
	cmd.SetStdout(&out)
	cmd.SetStderr(&syncBuffer{})
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("run: %v", runErr)
		}
	case <-time.After(ptyTestDeadline):
		t.Fatal("TTY.Run timed out")
	}
	got := strings.TrimSpace(out.String())
	if got != strconv.Itoa(parentPgid) {
		t.Errorf("child pgid = %s, want parent pgid %d (child must NOT be in a separate process group)", got, parentPgid)
	}
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

func TestPTYTeeTokenCapturedAndReturns(t *testing.T) {
	var tee syncBuffer
	var out syncBuffer
	cmd := NewPTYTee([]string{"/bin/sh", "-c", "echo sk-ant-oat01-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, &tee)
	cmd.SetStdout(&out)
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(ptyTestDeadline):
		t.Fatal("PTYTee.Run timed out — regression: blocked after child exit")
	}
	if !strings.Contains(tee.String(), "sk-ant-oat01-") {
		t.Fatalf("tee did not capture token: %q", tee.String())
	}
}

func TestPTYTeeGrandchildHoldingSlaveCantHang(t *testing.T) {
	var out syncBuffer
	cmd := NewPTYTee([]string{"/bin/sh", "-c", "sleep 30 & echo done"}, nil)
	cmd.SetStdout(&out)
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(ptyTestDeadline):
		t.Fatal("PTYTee.Run timed out — regression: grandchild holding slave fd blocked master close")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("PTYTee.Run took %v, want under 3s", elapsed)
	}
	if !strings.Contains(out.String(), "done") {
		t.Fatalf("stdout missing expected output: %q", out.String())
	}
}

func TestPTYTeeNonFileStdinPumpLifetime(t *testing.T) {
	release := make(chan struct{})
	readObservedRelease := make(chan struct{})

	r := &blockingReader{release: release, done: readObservedRelease}

	var out syncBuffer
	cmd := NewPTYTee([]string{"/bin/sh", "-c", "echo hi"}, nil)
	cmd.SetStdin(r)
	cmd.SetStdout(&out)

	runDone := make(chan error, 1)
	go func() { runDone <- cmd.Run() }()

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(ptyTestDeadline):
		t.Fatal("PTYTee.Run timed out before child exited")
	}

	close(release)

	select {
	case <-readObservedRelease:
	case <-time.After(3 * time.Second):
		t.Fatal("stdin copy goroutine did not observe release within 3s")
	}
}

type blockingReader struct {
	release <-chan struct{}
	done    chan<- struct{}
}

func (r *blockingReader) Read(_ []byte) (int, error) {
	<-r.release
	close(r.done)
	return 0, io.EOF
}

func TestTTYForwardWinchNoLeakAfterRun(t *testing.T) {
	tty := &TTY{Argv: []string{"/bin/sh", "-c", "exit 0"}}
	tty.SetStdin(strings.NewReader(""))
	tty.SetStdout(&syncBuffer{})
	tty.SetStderr(&syncBuffer{})

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	defer signal.Stop(ch)

	if err := tty.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatalf("kill SIGWINCH: %v", err)
	}
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("SIGWINCH not delivered to our channel after TTY.Run returned — signal.Stop may have affected it")
	}
}

func TestTTYForwardWinchForwardsToChild(t *testing.T) {
	script := `
winch_count=0
trap 'winch_count=$((winch_count+1)); printf "%d" "$winch_count"' WINCH
printf ready
read -r _line
`
	var out syncBuffer
	tty := &TTY{Argv: []string{"/bin/sh", "-c", script}}
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	tty.SetStdin(pr)
	tty.SetStdout(&out)
	tty.SetStderr(&syncBuffer{})

	runDone := make(chan error, 1)
	go func() { runDone <- tty.Run() }()

	deadline := time.After(ptyTestDeadline)
	for !strings.Contains(out.String(), "ready") {
		select {
		case <-deadline:
			t.Fatal("child never printed 'ready'")
		case <-time.After(5 * time.Millisecond):
		}
	}

	if err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH); err != nil {
		t.Fatalf("kill SIGWINCH self: %v", err)
	}

	deadline2 := time.After(ptyTestDeadline)
	for !strings.Contains(out.String(), "1") {
		select {
		case <-deadline2:
			_, _ = pw.WriteString("done\n")
			_ = pw.Close()
			t.Fatal("SIGWINCH not forwarded to child (child WINCH trap never fired)")
		case <-time.After(5 * time.Millisecond):
		}
	}

	_, _ = pw.WriteString("done\n")
	_ = pw.Close()
	select {
	case err := <-runDone:
		_ = err
	case <-time.After(ptyTestDeadline):
		t.Fatal("TTY.Run timed out")
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
