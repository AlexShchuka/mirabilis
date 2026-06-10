package app

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestFlowEscFromMenuQuitsNoHandoff(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	ctx := context.Background()
	m := newApp(ctx, &runner.FakeRunner{}, provision.Status{})
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(
		t,
		tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("mirabilis"))
		},
		teatest.WithDuration(3*time.Second),
		teatest.WithCheckInterval(10*time.Millisecond),
	)

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	final := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
	app, ok := final.(appModel)
	if !ok {
		t.Fatalf("FinalModel is %T, want appModel", final)
	}
	if app.handoff {
		t.Error("esc from menu should not set handoff")
	}
}

func TestFlowMenuGolden(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	ctx := context.Background()
	m := newApp(ctx, &runner.FakeRunner{}, provision.Status{})

	var acc bytes.Buffer
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(
		t,
		io.TeeReader(tm.Output(), &acc),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("mirabilis"))
		},
		teatest.WithDuration(3*time.Second),
		teatest.WithCheckInterval(10*time.Millisecond),
	)
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})

	rest, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	acc.Write(rest)

	teatest.RequireEqualOutput(t, lastFrame(acc.Bytes()))
}

func lastFrame(out []byte) []byte {
	if i := bytes.LastIndex(out, []byte("\x1b[H\x1b[2J")); i >= 0 {
		return out[i:]
	}
	return out
}
