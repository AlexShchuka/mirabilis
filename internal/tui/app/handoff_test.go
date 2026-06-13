//go:build darwin || linux

package app_test

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/tui/app"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
)

const (
	altScreenEnter = "\x1b[?1049h"
	altScreenLeave = "\x1b[?1049l"
)

func setRealExec(t *testing.T) {
	t.Helper()
	resetCaptured()
	app.SetExecRunner(tea.Exec)
	t.Cleanup(func() { app.SetExecRunner(captureExec) })
}

func terminalStep(name string, argv []string) *syncCommand {
	return &syncCommand{
		meta:    pipeline.Meta{Name: name, Title: "Attach", Kind: pipeline.Terminal},
		checkFn: func(_ context.Context) (bool, error) { return false, nil },
		runFn: func(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
			out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: name, Argv: argv}
			r := <-in
			if r.Cancelled {
				return pipeline.ErrCancelled
			}
			return nil
		},
	}
}

func TestHandoffRealExecRunsChildAndReturnsToMenu(t *testing.T) {
	setRealExec(t)
	f := newFakeFacade([]pipeline.Command{
		terminalStep("attach", []string{"/bin/sh", "-c", "printf handoff-child-ok"}),
	})
	tm := newApp(t, f)

	tm.Send(bus.MenuChosen{Action: "launch"})

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("handoff-child-ok"))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte(uistr.WelcomeHint))
	}, teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(20*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(10*time.Second))
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func (b *lockedBuffer) IndexAfter(offset int, needle string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	data := b.buf.Bytes()
	if offset > len(data) {
		return -1
	}
	i := bytes.Index(data[offset:], []byte(needle))
	if i < 0 {
		return -1
	}
	return offset + i + len(needle)
}

func waitContainsAfter(t *testing.T, b *lockedBuffer, offset int, needle string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if end := b.IndexAfter(offset, needle); end >= 0 {
			return end
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in pty output", needle)
	return -1
}

func TestHandoffRealPTYChildSeesTTYAndTermiosRestored(t *testing.T) {
	setRealExec(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "xterm")

	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	t.Cleanup(func() {
		_ = master.Close()
		_ = slave.Close()
	})
	if err := pty.Setsize(master, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
		t.Fatalf("pty.Setsize: %v", err)
	}

	before, err := unix.IoctlGetTermios(int(slave.Fd()), reqGetTermios)
	if err != nil {
		t.Fatalf("get termios before: %v", err)
	}

	f := newFakeFacade([]pipeline.Command{
		terminalStep("attach", []string{"/bin/sh", "-c", "test -t 0 && test -t 1 && printf in-child-tty || printf in-child-no-tty"}),
	})
	a := app.New(context.Background(), f, false)
	p := tea.NewProgram(a, tea.WithInput(slave), tea.WithOutput(slave))

	out := &lockedBuffer{}
	copyDone := make(chan struct{})
	go func() {
		defer close(copyDone)
		_, _ = io.Copy(out, master)
	}()

	runDone := make(chan error, 1)
	go func() {
		_, rerr := p.Run()
		runDone <- rerr
	}()

	launchOffset := waitContainsAfter(t, out, 0, uistr.WelcomeHint)
	if out.IndexAfter(0, altScreenEnter) < 0 {
		t.Errorf("alt-screen enter sequence %q not written at startup", altScreenEnter)
	}
	p.Send(bus.MenuChosen{Action: "launch"})

	waitContainsAfter(t, out, launchOffset, "in-child-tty")

	quitDeadline := time.After(15 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	p.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	for {
		select {
		case rerr := <-runDone:
			if rerr != nil {
				t.Fatalf("program run: %v", rerr)
			}
		case <-ticker.C:
			p.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
			continue
		case <-quitDeadline:
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			t.Fatalf("program did not return to a quittable menu after handoff\n%s", buf[:n])
		}
		break
	}

	after, err := unix.IoctlGetTermios(int(slave.Fd()), reqGetTermios)
	if err != nil {
		t.Fatalf("get termios after: %v", err)
	}
	if before.Lflag&unix.ECHO != after.Lflag&unix.ECHO {
		t.Errorf("ECHO not restored: before=%x after=%x", before.Lflag&unix.ECHO, after.Lflag&unix.ECHO)
	}
	if before.Lflag&unix.ICANON != after.Lflag&unix.ICANON {
		t.Errorf("ICANON not restored: before=%x after=%x", before.Lflag&unix.ICANON, after.Lflag&unix.ICANON)
	}

	if out.IndexAfter(0, altScreenLeave) < 0 {
		t.Errorf("alt-screen leave sequence %q not written on quit", altScreenLeave)
	}

	_ = slave.Close()
	select {
	case <-copyDone:
	case <-time.After(5 * time.Second):
		t.Error("pty reader did not stop after slave close")
	}
}
