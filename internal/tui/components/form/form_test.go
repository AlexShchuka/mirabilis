package form_test

import (
	"bytes"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/components/form"
)

type harness struct {
	form      form.Model
	result    []string
	gotResult bool
	popped    bool
}

func (h harness) Init() tea.Cmd { return h.form.Init() }

func (h harness) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.ScreenResult:
		h.gotResult = true
		if v, ok := msg.Value.([]string); ok {
			h.result = v
		}
		return h, tea.Quit
	case bus.ScreenPop:
		h.popped = true
		return h, tea.Quit
	}
	var cmd tea.Cmd
	h.form, cmd = h.form.Update(msg)
	return h, cmd
}

func (h harness) View() tea.View { return tea.NewView(h.form.View()) }

func run(t *testing.T, h harness, keys ...tea.KeyPressMsg) harness {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	tm := teatest.NewTestModel(t, h, teatest.WithInitialTermSize(80, 24))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("alpha"))
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(10*time.Millisecond))

	for _, k := range keys {
		tm.Send(k)
	}
	final, ok := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(harness)
	if !ok {
		t.Fatal("FinalModel is not harness")
	}
	return final
}

func TestSubmitEmitsScreenResult(t *testing.T) {
	h := run(t,
		harness{form: form.NewMultiSelect("Plugins", []string{"alpha", "beta"}, nil)},
		tea.KeyPressMsg{Code: 'x', Text: "x"},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	if !h.gotResult {
		t.Fatal("submit did not emit bus.ScreenResult")
	}
	if h.popped {
		t.Fatal("submit also emitted bus.ScreenPop")
	}
	if len(h.result) != 1 || h.result[0] != "alpha" {
		t.Fatalf("result = %v, want [alpha]", h.result)
	}
}

func TestSubmitKeepsPreselected(t *testing.T) {
	h := run(t,
		harness{form: form.NewMultiSelect("Plugins", []string{"alpha", "beta"}, []string{"beta"})},
		tea.KeyPressMsg{Code: tea.KeyEnter},
	)
	if !h.gotResult {
		t.Fatal("submit did not emit bus.ScreenResult")
	}
	if len(h.result) != 1 || h.result[0] != "beta" {
		t.Fatalf("result = %v, want [beta]", h.result)
	}
}

func TestEscEmitsScreenPop(t *testing.T) {
	h := run(t,
		harness{form: form.NewMultiSelect("Plugins", []string{"alpha", "beta"}, nil)},
		tea.KeyPressMsg{Code: tea.KeyEscape},
	)
	if !h.popped {
		t.Fatal("esc did not emit bus.ScreenPop")
	}
	if h.gotResult {
		t.Fatal("esc also emitted bus.ScreenResult")
	}
}

func TestCtrlCEmitsScreenPop(t *testing.T) {
	h := run(t,
		harness{form: form.NewMultiSelect("Plugins", []string{"alpha", "beta"}, nil)},
		tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl},
	)
	if !h.popped {
		t.Fatal("ctrl+c did not emit bus.ScreenPop")
	}
}
