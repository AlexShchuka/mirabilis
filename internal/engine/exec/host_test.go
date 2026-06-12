//go:build darwin || linux

package exec

import (
	"context"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func splitStreams(t *testing.T, evs []Event) (stdout, stderr []string, exited Event) {
	t.Helper()
	if len(evs) == 0 {
		t.Fatal("no events")
	}
	if evs[0].Kind != KindStarted {
		t.Fatalf("first event = %v, want KindStarted", evs[0].Kind)
	}
	exitedCount := 0
	for _, ev := range evs {
		switch ev.Kind {
		case KindStdout:
			stdout = append(stdout, ev.Line)
		case KindStderr:
			stderr = append(stderr, ev.Line)
		case KindExited:
			exited = ev
			exitedCount++
		}
	}
	if exitedCount != 1 {
		t.Fatalf("KindExited count = %d, want 1", exitedCount)
	}
	if evs[len(evs)-1].Kind != KindExited {
		t.Fatalf("last event = %v, want KindExited", evs[len(evs)-1].Kind)
	}
	return stdout, stderr, exited
}

func TestHostStreamContract(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "echo out; echo err >&2"}
	evs := drain(NewHost().Stream(context.Background(), Spec{Argv: argv}))

	if got := evs[0].Argv; strings.Join(got, " ") != strings.Join(argv, " ") {
		t.Fatalf("started argv = %v, want %v", got, argv)
	}
	stdout, stderr, exited := splitStreams(t, evs)
	if want := []string{"out"}; !equalLines(stdout, want) {
		t.Fatalf("stdout = %v, want %v", stdout, want)
	}
	if want := []string{"err"}; !equalLines(stderr, want) {
		t.Fatalf("stderr = %v, want %v", stderr, want)
	}
	if exited.Code != 0 || exited.Err != nil {
		t.Fatalf("exited = %+v, want code 0 nil err", exited)
	}
}

func TestHostStdoutOrder(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "printf 'l1\\nl2\\nl3\\n'"}
	stdout, _, _ := splitStreams(t, drain(NewHost().Stream(context.Background(), Spec{Argv: argv})))
	if want := []string{"l1", "l2", "l3"}; !equalLines(stdout, want) {
		t.Fatalf("stdout = %v, want %v", stdout, want)
	}
}

func TestHostExitCode(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "exit 3"}
	_, _, exited := splitStreams(t, drain(NewHost().Stream(context.Background(), Spec{Argv: argv})))
	if exited.Code != 3 {
		t.Fatalf("exit code = %d, want 3", exited.Code)
	}
	if exited.Err == nil {
		t.Fatal("exit err = nil, want non-nil")
	}
}

func TestHostStartFailure(t *testing.T) {
	evs := drain(NewHost().Stream(context.Background(), Spec{Argv: []string{"/no/such/binary-xyz"}}))
	if evs[0].Kind != KindStarted {
		t.Fatalf("first event = %v, want KindStarted", evs[0].Kind)
	}
	_, _, exited := splitStreams(t, evs)
	if exited.Code != -1 {
		t.Fatalf("exit code = %d, want -1", exited.Code)
	}
	if exited.Err == nil {
		t.Fatal("exit err = nil, want non-nil")
	}
}

func TestHostLongLine(t *testing.T) {
	line := strings.Repeat("x", 200000)
	argv := []string{"/bin/sh", "-c", "cat"}
	spec := Spec{Argv: argv, Stdin: strings.NewReader(line + "\n")}
	stdout, _, exited := splitStreams(t, drain(NewHost().Stream(context.Background(), spec)))
	if len(stdout) != 1 {
		t.Fatalf("stdout lines = %d, want 1", len(stdout))
	}
	if stdout[0] != line {
		t.Fatalf("stdout len = %d, want %d", len(stdout[0]), len(line))
	}
	if exited.Code != 0 {
		t.Fatalf("exit code = %d, want 0", exited.Code)
	}
}

func TestHostStdin(t *testing.T) {
	argv := []string{"/bin/cat"}
	spec := Spec{Argv: argv, Stdin: strings.NewReader("hello\n")}
	stdout, _, _ := splitStreams(t, drain(NewHost().Stream(context.Background(), spec)))
	if want := []string{"hello"}; !equalLines(stdout, want) {
		t.Fatalf("stdout = %v, want %v", stdout, want)
	}
}

func TestHostEnv(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "echo $FOO"}
	spec := Spec{Argv: argv, Env: []string{"FOO=bar"}}
	stdout, _, _ := splitStreams(t, drain(NewHost().Stream(context.Background(), spec)))
	if want := []string{"bar"}; !equalLines(stdout, want) {
		t.Fatalf("stdout = %v, want %v", stdout, want)
	}
}

func TestHostDir(t *testing.T) {
	dir := t.TempDir()
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	argv := []string{"/bin/sh", "-c", "pwd -P"}
	stdout, _, _ := splitStreams(t, drain(NewHost().Stream(context.Background(), Spec{Argv: argv, Dir: dir})))
	if len(stdout) != 1 {
		t.Fatalf("stdout lines = %d, want 1", len(stdout))
	}
	got, err := filepath.EvalSymlinks(strings.TrimSpace(stdout[0]))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("pwd = %q, want %q", got, want)
	}
}

func TestHostCtxKillsGrandchild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	argv := []string{"/bin/sh", "-c", "sleep 30 & echo $!; wait"}
	ch := NewHost().Stream(ctx, Spec{Argv: argv})

	var pid int
	for ev := range ch {
		if ev.Kind == KindStdout {
			p, err := strconv.Atoi(strings.TrimSpace(ev.Line))
			if err != nil {
				t.Fatalf("parse grandchild pid %q: %v", ev.Line, err)
			}
			pid = p
			break
		}
	}
	if pid == 0 {
		t.Fatal("did not capture grandchild pid")
	}

	start := time.Now()
	cancel()
	for range ch {
	}
	if elapsed := time.Since(start); elapsed > waitDelay {
		t.Fatalf("stream took %v after cancel, want < %v", elapsed, waitDelay)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("grandchild %d still alive", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestHostNoGoroutineLeak(t *testing.T) {
	base := runtime.NumGoroutine()
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		ch := NewHost().Stream(ctx, Spec{Argv: []string{"/bin/sh", "-c", "sleep 30"}})
		<-ch
		cancel()
		for range ch {
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			t.Fatalf("goroutine leak: base=%d now=%d", base, runtime.NumGoroutine())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestHostOverLongLineSurfacesScanError(t *testing.T) {
	line := strings.Repeat("x", scanLineMax+1)
	argv := []string{"/bin/sh", "-c", "cat"}
	spec := Spec{Argv: argv, Stdin: strings.NewReader(line + "\n")}
	evs := drain(NewHost().Stream(context.Background(), spec))
	var stderrLines []string
	for _, ev := range evs {
		if ev.Kind == KindStderr {
			stderrLines = append(stderrLines, ev.Line)
		}
	}
	if len(stderrLines) == 0 {
		t.Fatal("expected scan error diagnostic on stderr, got none")
	}
	found := false
	for _, l := range stderrLines {
		if strings.Contains(l, "scan error") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("stderr lines = %v, want one containing 'scan error'", stderrLines)
	}
}

func TestRunCollectsStdout(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "printf 'a\\nb\\n'"}
	out, err := Run(context.Background(), NewHost(), Spec{Argv: argv})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if out != "a\nb" {
		t.Fatalf("Run out = %q, want %q", out, "a\nb")
	}
}

func TestRunFailureIncludesStderr(t *testing.T) {
	argv := []string{"/bin/sh", "-c", "echo boom >&2; exit 1"}
	_, err := Run(context.Background(), NewHost(), Spec{Argv: argv})
	if err == nil {
		t.Fatal("Run err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Run err = %v, want it to contain %q", err, "boom")
	}
}
