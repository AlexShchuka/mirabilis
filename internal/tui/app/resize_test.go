//go:build darwin || linux

package app_test

import (
	"context"
	"io"
	"os"
	"runtime"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"

	"github.com/AlexShchuka/mirabilis/internal/tui/app"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

type ptyHarness struct {
	prog    *tea.Program
	out     *lockedBuffer
	master  *os.File
	runErr  chan error
	stopped chan struct{}
}

func startPTYApp(t *testing.T, f *fakeFacade, rows, cols int) *ptyHarness {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm")

	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	if err := pty.Setsize(master, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		t.Fatalf("pty.Setsize: %v", err)
	}

	a := app.New(context.Background(), f)
	p := tea.NewProgram(a, tea.WithInput(slave), tea.WithOutput(slave))

	out := &lockedBuffer{}
	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		_, _ = io.Copy(out, master)
	}()

	runErr := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		_, rerr := p.Run()
		runErr <- rerr
		close(stopped)
	}()

	h := &ptyHarness{prog: p, out: out, master: master, runErr: runErr, stopped: stopped}

	t.Cleanup(func() {
		p.Quit()
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Errorf("pty program did not stop after Quit\n%s", buf[:n])
		}
		p.Wait()
		_ = slave.Close()
		_ = master.Close()
		<-copyDone
	})

	return h
}

func (h *ptyHarness) waitStopped(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case <-h.stopped:
		h.prog.Wait()
		if rerr := <-h.runErr; rerr != nil {
			t.Fatalf("program run: %v", rerr)
		}
	case <-time.After(d):
		t.Fatal("program did not quit within deadline")
	}
}

func TestRealBytesNavMovesCursorAndEnterDispatches(t *testing.T) {
	f := newFakeFacade(nil)
	h := startPTYApp(t, f, 40, 120)

	waitContainsAfter(t, h.out, 0, uistr.WelcomeHint)

	for range 3 {
		_, _ = h.master.Write([]byte("\x1b[B"))
		time.Sleep(20 * time.Millisecond)
	}
	_, _ = h.master.Write([]byte("\r"))

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		called := false
		for _, c := range f.getCallLog() {
			if c == "OpenVSCode" {
				called = true
			}
		}
		if called {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("real-byte nav+enter did not dispatch the VS Code action; call log = %v", f.getCallLog())
}

func TestResizeKeepsAltScreenAndReflows(t *testing.T) {
	f := newFakeFacade(nil)
	h := startPTYApp(t, f, 40, 120)

	waitContainsAfter(t, h.out, 0, uistr.WelcomeHint)
	if h.out.IndexAfter(0, altScreenEnter) < 0 {
		t.Fatalf("alt-screen enter %q not present at startup", altScreenEnter)
	}

	resizeProbe := h.out.Len()

	resize := func(cols, rows int, token string) {
		t.Helper()
		probe := h.out.Len()
		h.prog.Send(tea.WindowSizeMsg{Width: cols, Height: rows})
		waitContainsAfter(t, h.out, probe, token)
	}

	resize(30, 8, uistr.TooSmall)
	resize(50, 20, uistr.WelcomeHint)
	resize(28, 9, uistr.TooSmall)
	resize(100, 36, uistr.WelcomeHint)

	if h.out.IndexAfter(resizeProbe, altScreenLeave) >= 0 {
		t.Errorf("alt-screen left (%q seen) during resizing — frame fell back to inline (stranded-frame regression)", altScreenLeave)
	}

	h.prog.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	h.waitStopped(t, 10*time.Second)

	waitContainsAfter(t, h.out, 0, altScreenLeave)
}
