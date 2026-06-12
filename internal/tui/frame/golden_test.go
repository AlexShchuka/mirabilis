package frame_test

import (
	"bytes"
	"io"
	"regexp"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/frame"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;<=>?]*[ -/]*[@-~]")

func stripANSI(b []byte) []byte {
	return ansiPattern.ReplaceAll(b, nil)
}

type appHarness struct {
	fr   frame.Model
	menu screens.Menu
}

func (h appHarness) Init() tea.Cmd { return h.menu.Init() }

func (h appHarness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if mc, ok := msg.(bus.MenuChosen); ok && mc.Action == screens.ActionQuit {
		return h, tea.Quit
	}
	var cmds []tea.Cmd
	var cmd tea.Cmd
	h.fr, cmd = h.fr.Update(msg)
	cmds = append(cmds, cmd)
	var scr router.Screen
	scr, cmd = h.menu.Update(msg)
	h.menu = scr.(screens.Menu)
	cmds = append(cmds, cmd)
	return h, tea.Batch(cmds...)
}

func (h appHarness) View() tea.View {
	return tea.NewView(h.fr.View(h.menu.View()))
}

func TestFrameMenuGolden(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")

	fr := frame.New("mirabilis", "v1.3.0", screens.MenuItems())
	fr, _ = fr.Update(bus.StatusChanged{Snapshot: obs.Snapshot{
		"container": {State: obs.StateOK, Detail: "up"},
		"harness":   {State: obs.StateOK, Detail: "on"},
		"proxy":     {State: obs.StateOK, Detail: "on"},
	}})
	h := appHarness{fr: fr, menu: screens.NewMenu("app/menu")}

	var acc bytes.Buffer
	tm := teatest.NewTestModel(t, h, teatest.WithInitialTermSize(80, 24))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, io.TeeReader(tm.Output(), &acc), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("select an action on the left"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(10*time.Millisecond))

	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})

	rest, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	acc.Write(rest)

	teatest.RequireEqualOutput(t, stripANSI(lastFrame(acc.Bytes())))
}

func lastFrame(out []byte) []byte {
	if i := bytes.LastIndex(out, []byte("\x1b[H\x1b[2J")); i >= 0 {
		return out[i:]
	}
	return out
}
