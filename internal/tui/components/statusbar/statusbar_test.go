package statusbar

import (
	"regexp"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func TestViewEmpty(t *testing.T) {
	if got := New().View(); got != "" {
		t.Errorf("View() = %q, want empty", got)
	}
}

func TestViewOK(t *testing.T) {
	m := New().Update(bus.StatusChanged{Snapshot: obs.Snapshot{
		"container": {State: obs.StateOK, Detail: "up"},
		"harness":   {State: obs.StateOK, Detail: "on"},
		"proxy":     {State: obs.StateOK, Detail: "on"},
	}})
	got := plain(m.View())
	want := "container up · harness on · proxy on"
	if got != want {
		t.Errorf("View() = %q, want %q", got, want)
	}
}

func TestViewDegraded(t *testing.T) {
	m := New().Update(bus.StatusChanged{Snapshot: obs.Snapshot{
		"container": {State: obs.StateOK, Detail: "up"},
		"notify":    {State: obs.StateDegraded, Detail: "send failed"},
		"proxy":     {State: obs.StateDegraded},
	}})
	got := plain(m.View())
	if !strings.Contains(got, "degraded: notify, proxy") {
		t.Errorf("View() = %q, want degraded segment with notify, proxy", got)
	}
	if !strings.HasPrefix(got, "container up") {
		t.Errorf("View() = %q, want ok segment first", got)
	}
}

func TestViewDetailFallback(t *testing.T) {
	m := New().Update(bus.StatusChanged{Snapshot: obs.Snapshot{
		"harness": {State: obs.StateOff},
	}})
	if got := plain(m.View()); got != "harness off" {
		t.Errorf("View() = %q, want %q", got, "harness off")
	}
}

func TestUpdateIgnoresOtherMsgs(t *testing.T) {
	m := New().Update(bus.StatusChanged{Snapshot: obs.Snapshot{
		"container": {State: obs.StateOK, Detail: "up"},
	}})
	m = m.Update(bus.ScreenPop{})
	if got := plain(m.View()); got != "container up" {
		t.Errorf("View() = %q, want snapshot kept", got)
	}
}
