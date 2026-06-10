package ghauth

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func devcontainerShim(t *testing.T, body string) {
	t.Helper()
	shimDir := t.TempDir()
	if err := os.WriteFile(shimDir+"/devcontainer", []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir)
}

func TestRun_SuccessfulStart_PumpsOutput(t *testing.T) {
	devcontainerShim(t, "echo waiting-for-code\nexit 0\n")
	g := New(context.Background(), &runner.FakeRunner{RepoVal: t.TempDir()}, 80, 24)
	g.linesCh = make(chan string, 10)
	g.doneCh = make(chan error, 1)

	done := make(chan struct{})
	go func() { defer close(done); g.run() }()

	var lines []string
	timeout := time.After(2 * time.Second)
collect:
	for {
		select {
		case line, ok := <-g.linesCh:
			if !ok {
				break collect
			}
			lines = append(lines, line)
		case <-timeout:
			t.Fatal("run() did not drain after the command exited")
		}
	}
	<-done
	if waitErr := <-g.doneCh; waitErr != nil {
		t.Errorf("doneCh = %v, want nil for a clean exit", waitErr)
	}
	if len(lines) != 1 || lines[0] != "waiting-for-code" {
		t.Errorf("pumped lines = %v, want [waiting-for-code]", lines)
	}
}

func TestLaunch_StartsRunAndReturnsFirstLine(t *testing.T) {
	devcontainerShim(t, "echo first-line\nexit 0\n")
	g := New(context.Background(), &runner.FakeRunner{RepoVal: t.TempDir()}, 80, 24)
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)

	type result struct{ msg tea.Msg }
	res := make(chan result, 1)
	go func() { res <- result{msg: g.launch()()} }()

	select {
	case r := <-res:
		line, ok := r.msg.(LineMsg)
		if !ok {
			t.Fatalf("launch produced %T, want LineMsg", r.msg)
		}
		if string(line) != "first-line" {
			t.Errorf("first LineMsg = %q, want first-line", string(line))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("launch did not yield the first line")
	}
}

func TestModelUpdate_LineMsg_AppendsLine(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string, 1)
	g2, cmd := g.Update(LineMsg("! one-time code: ABCD-1234"))
	if cmd == nil {
		t.Fatal("LineMsg should schedule the next read")
	}
	if len(g2.lines) != 1 || g2.code != "ABCD-1234" {
		t.Errorf("after LineMsg: lines=%v code=%q, want the line captured and code parsed", g2.lines, g2.code)
	}
}

func TestOpenBrowserCmd_RunsHostBrowser(t *testing.T) {
	shimDir := t.TempDir()
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	if err := os.WriteFile(shimDir+"/"+opener, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir)
	cmd := openBrowserCmd("https://example.com")
	if cmd == nil {
		t.Fatal("openBrowserCmd returned nil")
	}
	msg, ok := cmd().(browserMsg)
	if !ok {
		t.Fatalf("openBrowserCmd emitted %T, want browserMsg", cmd())
	}
	if msg.err != nil {
		t.Errorf("browserMsg.err = %v, want nil when the shim launches cleanly", msg.err)
	}
}
