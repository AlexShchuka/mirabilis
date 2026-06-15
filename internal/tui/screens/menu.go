package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/a11y"
	"github.com/AlexShchuka/mirabilis/internal/tui/frame"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

const (
	ActionLaunch  = "launch"
	ActionHarness = "harness"
	ActionVSCode  = "vscode"
	ActionReset   = "reset"
	ActionQuit    = "quit"
)

func MenuItems() []frame.Item {
	return []frame.Item{
		{Title: uistr.MenuActionLaunch, Desc: uistr.MenuDescLaunch, Action: ActionLaunch, Enabled: true},
		{Title: uistr.MenuActionHarness, Desc: uistr.MenuDescHarness, Action: ActionHarness, Enabled: true},
		{Title: uistr.MenuActionVSCode, Desc: uistr.MenuDescVSCode, Action: ActionVSCode, Enabled: true},
		{Title: uistr.MenuActionReset, Desc: uistr.MenuDescReset, Action: ActionReset, Enabled: true},
		{Title: uistr.MenuActionQuit, Action: ActionQuit, Enabled: true},
	}
}

type Menu struct {
	id          bus.NodeID
	notice      string
	errText     string
	width       int
	height      int
	chromeFrame int
}

func NewMenu(id bus.NodeID) Menu {
	return Menu{id: id}
}

func (m Menu) WithNotice(notice string) Menu {
	m.notice = notice
	return m
}

func (m Menu) WithError(errText string) Menu {
	m.errText = errText
	return m
}

func (m Menu) Notice() string {
	return m.notice
}

func (m Menu) ErrText() string {
	return m.errText
}

func (m Menu) ID() bus.NodeID {
	return m.id
}

func (m Menu) Init() tea.Cmd {
	return nil
}

func (m Menu) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, quitCmd
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case bus.ChromeTick:
		m.chromeFrame = msg.Frame
	case bus.Envelope:
		return m.Update(msg.Msg)
	}
	return m, nil
}

func quitCmd() tea.Msg {
	return bus.MenuChosen{Action: ActionQuit}
}

var logoLargeFrames = []string{
	uistr.LogoLargeFrameA,
	uistr.LogoLargeFrameB,
	uistr.LogoLargeFrameC,
	uistr.LogoLargeFrameD,
	uistr.LogoLargeFrameE,
	uistr.LogoLargeFrameF,
	uistr.LogoLargeFrameG,
	uistr.LogoLargeFrameH,
}

func (m Menu) largeMark() string {
	if a11y.ReducedMotion() {
		return uistr.LogoLargeStatic
	}
	return logoLargeFrames[m.chromeFrame%len(logoLargeFrames)]
}

func (m Menu) showLargeMark() bool {
	return m.width >= frame.NarrowWidth && m.height >= frame.ShortHeight
}

func (m Menu) View() string {
	lines := []string{}
	if m.showLargeMark() {
		for _, l := range strings.Split(m.largeMark(), "\n") {
			lines = append(lines, " "+styles.Spinner.Render(l))
		}
		lines = append(lines, "")
	}
	lines = append(lines,
		" "+styles.Title.Render(uistr.AppName),
		"",
		" "+styles.Hint.Render(uistr.WelcomeHint),
	)
	if m.notice != "" && m.notice != m.errText {
		lines = append(lines, "", " "+styles.Degraded.Render(m.notice))
	}
	if m.errText != "" {
		lines = append(lines,
			"",
			" "+styles.Degraded.Render(uistr.GlyphError+" "+m.errText),
			" "+styles.Hint.Render(uistr.ErrorDismissHint),
		)
	}
	return strings.Join(lines, "\n")
}
