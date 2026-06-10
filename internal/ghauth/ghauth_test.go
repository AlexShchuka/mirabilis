package ghauth

import (
	"context"
	"io"
	"os"
	gort "runtime"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/ui"
)

func TestParseUserCode(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{give: "! First copy your one-time code: ABCD-1234", want: "ABCD-1234"},
		{give: "code: 2741-EE59 now", want: "2741-EE59"},
		{give: "no code here", want: ""},
		{give: "lowercase abcd-1234 is ignored", want: ""},
	}
	for _, tt := range tests {
		if got := ParseUserCode(tt.give); got != tt.want {
			t.Errorf("ParseUserCode(%q) = %q, want %q", tt.give, got, tt.want)
		}
	}
}

func TestParseDeviceURL(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{give: "Open https://github.com/login/device to continue", want: "https://github.com/login/device"},
		{give: "go to https://github.com/login/device.", want: "https://github.com/login/device"},
		{give: "no url", want: ""},
	}
	for _, tt := range tests {
		if got := ParseDeviceURL(tt.give); got != tt.want {
			t.Errorf("ParseDeviceURL(%q) = %q, want %q", tt.give, got, tt.want)
		}
	}
}

func TestLoginArgsRequestWorkflowScope(t *testing.T) {
	args := loginArgs()
	has := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}
	if !has("gh") || !has("auth") || !has("login") {
		t.Fatalf("loginArgs is not a gh auth login invocation: %v", args)
	}
	scoped := false
	for i, a := range args {
		if a == "--scopes" && i+1 < len(args) && args[i+1] == "workflow" {
			scoped = true
		}
	}
	if !scoped {
		t.Errorf("loginArgs must request the workflow scope, got %v", args)
	}
}

func TestRunExitsCleanlyOnStartError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g := New(ctx, &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.run()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("run() did not return after process start failure")
	}
}

func TestPumpWritesDoneChOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	g := New(ctx, &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)

	pr, pw := io.Pipe()
	go func() { _, _ = pw.Write([]byte("waiting for one-time code...\n")) }()

	go g.pump(pr, func() error { return nil })

	cancel()

	select {
	case err := <-g.doneCh:
		if err != context.Canceled {
			t.Errorf("doneCh = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not write doneCh after cancel — readNext would block and leak the goroutine")
	}
}

func TestOnLineCapturesAndOpensOnce(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)

	g.onLine("! First copy your one-time code: ABCD-1234")
	if g.code != "ABCD-1234" {
		t.Errorf("code = %q, want ABCD-1234", g.code)
	}
	if g.opened {
		t.Error("must not open the browser before the URL is known")
	}

	g.onLine("Open https://github.com/login/device")
	if g.url != "https://github.com/login/device" {
		t.Errorf("url = %q, want the device URL", g.url)
	}
	if !g.opened {
		t.Error("should open the browser once both code and URL are known")
	}
}

func TestModelUpdate_WindowSizeMsg(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g2, cmd := g.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if g2.w != 100 || g2.h != 40 {
		t.Errorf("after WindowSizeMsg w=%d h=%d, want 100 40", g2.w, g2.h)
	}
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil cmd")
	}
}

func TestModelUpdate_SpinnerTickMsg(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g2, cmd := g.Update(spinner.TickMsg{})
	if g2 == nil {
		t.Fatal("Update returned nil")
	}
	_ = cmd
}

func TestModelUpdate_BrowserMsgError(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g2, _ := g.Update(browserMsg{err: context.Canceled})
	if g2.status != ui.GHAuthStatusNoOpen {
		t.Errorf("status = %q, want %q", g2.status, ui.GHAuthStatusNoOpen)
	}
}

func TestModelUpdate_BrowserMsgNoError(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g.status = ui.GHAuthStatusOpened
	g2, _ := g.Update(browserMsg{err: nil})
	if g2.status != ui.GHAuthStatusOpened {
		t.Errorf("status changed on nil browser err, got %q", g2.status)
	}
}

func TestModelUpdate_ExitMsg(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)
	g2, cmd := g.Update(ExitMsg{Err: nil})
	if !g2.finished {
		t.Error("ExitMsg should set finished")
	}
	if cmd == nil {
		t.Error("ExitMsg should return a cmd (DoneMsg emit)")
	}
	msg := cmd()
	if _, ok := msg.(DoneMsg); !ok {
		t.Errorf("ExitMsg cmd emits %T, want DoneMsg", msg)
	}
}

func TestModelUpdate_UnknownMsg(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g2, cmd := g.Update("ignored")
	if g2 == nil {
		t.Fatal("Update returned nil on unknown msg")
	}
	if cmd != nil {
		t.Error("unknown msg should return nil cmd")
	}
}

func TestModelView_CodeAndURL(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g.code = "ABCD-1234"
	g.url = "https://github.com/login/device"
	g.status = ui.GHAuthStatusOpened
	v := g.View()
	if !strings.Contains(v, ui.GHAuthTitle) {
		t.Errorf("View missing title, got:\n%s", v)
	}
	if !strings.Contains(v, "ABCD-1234") {
		t.Errorf("View missing code, got:\n%s", v)
	}
	if !strings.Contains(v, "https://github.com/login/device") {
		t.Errorf("View missing URL, got:\n%s", v)
	}
	if !strings.Contains(v, ui.GHAuthStatusOpened) {
		t.Errorf("View missing status, got:\n%s", v)
	}
}

func TestModelView_NoCodeYet(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	v := g.View()
	if !strings.Contains(v, ui.GHAuthTitle) {
		t.Errorf("View missing title, got:\n%s", v)
	}
	if strings.Contains(v, ui.GHAuthLabelCode) {
		t.Errorf("View should not show code section before code is known, got:\n%s", v)
	}
}

func TestModelInit_CreateChannels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g := New(ctx, &runner.FakeRunner{}, 80, 24)
	cmd := g.Init()
	if g.linesCh == nil {
		t.Error("Init should create linesCh")
	}
	if g.doneCh == nil {
		t.Error("Init should create doneCh")
	}
	if cmd == nil {
		t.Error("Init should return a non-nil batch cmd")
	}
	cancel()
}

func TestReadNext_ReceivesLine(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string, 1)
	g.doneCh = make(chan error, 1)
	g.linesCh <- "hello-line"
	msg := g.readNext()
	lm, ok := msg.(LineMsg)
	if !ok {
		t.Fatalf("readNext returned %T, want LineMsg", msg)
	}
	if string(lm) != "hello-line" {
		t.Errorf("readNext = %q, want hello-line", string(lm))
	}
}

func TestReadNext_ClosedChannelReturnsExitMsg(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)
	g.doneCh <- nil
	close(g.linesCh)
	msg := g.readNext()
	_, ok := msg.(ExitMsg)
	if !ok {
		t.Fatalf("readNext on closed chan returned %T, want ExitMsg", msg)
	}
}

func TestWaitLine_ProducesReadNext(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string, 1)
	g.doneCh = make(chan error, 1)
	g.linesCh <- "via-waitline"
	cmd := g.waitLine()
	if cmd == nil {
		t.Fatal("waitLine returned nil cmd")
	}
	msg := cmd()
	if lm, ok := msg.(LineMsg); !ok || string(lm) != "via-waitline" {
		t.Errorf("waitLine cmd() = %v, want LineMsg(via-waitline)", msg)
	}
}

func TestOpenHostBrowser_RunsShim(t *testing.T) {
	shimDir := t.TempDir()
	shimPath := shimDir + "/xdg-open"
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := os.Getenv("PATH")
	t.Setenv("PATH", shimDir+":"+base)
	if err := openHostBrowser("https://example.com"); err != nil {
		t.Errorf("openHostBrowser: %v", err)
	}
}

func TestOpenHostBrowser_WslviewFallback(t *testing.T) {
	if gort.GOOS == "darwin" {
		t.Skip("darwin uses open, not the wslview fallback")
	}
	shimDir := t.TempDir()
	if err := os.WriteFile(shimDir+"/wslview", []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir)
	if err := openHostBrowser("https://example.com"); err != nil {
		t.Errorf("openHostBrowser with only wslview on PATH: %v", err)
	}
}

func TestPumpScannerEOF_WritesNilDone(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string, 100)
	g.doneCh = make(chan error, 1)

	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte("line1\nline2\n"))
		pw.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.pump(pr, func() error { return nil })
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not finish after pipe closed")
	}
	if err := <-g.doneCh; err != nil {
		t.Errorf("doneCh = %v, want nil", err)
	}
}

func TestContextCancelStopsPump(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	g := New(ctx, &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)

	pr, pw := io.Pipe()
	go func() {
		for i := 0; i < 100; i++ {
			if _, err := pw.Write([]byte("line\n")); err != nil {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		_ = pw.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.pump(pr, func() error { return nil })
	}()

	<-g.linesCh
	cancel()

	select {
	case err := <-g.doneCh:
		if err != context.Canceled {
			t.Errorf("doneCh = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pump did not stop after context cancel")
	}
}
