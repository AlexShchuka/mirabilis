package frame_test

import (
	"regexp"
	"testing"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/frame"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;<=>?]*[ -/]*[@-~]")

func stripANSI(b []byte) []byte {
	return ansiPattern.ReplaceAll(b, nil)
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
	fr, _ = fr.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	menu := screens.NewMenu("app/menu")
	scr, _ := menu.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	menu = scr.(screens.Menu)

	teatest.RequireEqualOutput(t, stripANSI([]byte(fr.View(menu.View()))))
}
