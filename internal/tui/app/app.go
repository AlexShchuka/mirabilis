package app

import (
	"context"
	"io"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/a11y"
	"github.com/AlexShchuka/mirabilis/internal/tui/frame"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type Facade interface {
	LaunchSteps() []pipeline.Command
	Version() string
	Logger() *slog.Logger
	StatusUpdates() <-chan obs.Snapshot
	OnTokenExtracted(token string)
	NewTokenTee() (io.Writer, func() (string, bool))
	SaveMemory(ctx context.Context) error
	ResetSandbox(ctx context.Context) error
	ConfigureTelegram(ctx context.Context, token string) error
	HarnessStatus(ctx context.Context) (string, error)
	ApplyHarness(ctx context.Context, choice string) error
	OpenVSCode(ctx context.Context) error
	OpenURL(ctx context.Context, url string) error
	CopyText(ctx context.Context, text string) error
	LastHarnessChoice() string
	RememberHarnessChoice(choice string) error
	TelegramConfigured() bool
	MarkTelegramConfigured() error
}

var execRunner = tea.Exec

const chromePeriod = 100 * time.Millisecond

type chromeTickMsg struct {
	gen int
}

type App struct {
	ctx             context.Context //nolint:containedctx
	cancel          context.CancelFunc
	facade          Facade
	statusCh        <-chan obs.Snapshot
	frame           frame.Model
	router          router.Model
	pipe            *pipeline.Pipeline
	waiting         string
	menuAction      string
	winW            int
	winH            int
	launchCancelled bool
	busy            bool
	busyStarted     time.Time
	busyFrame       int
	busyGen         int
	chromeFrame     int
	chromeGen       int
	harnessChoice   string
	errNotice       string
}

func New(ctx context.Context, f Facade) App {
	ctx, cancel := context.WithCancel(ctx)
	menu := screens.NewMenu("app/menu")
	return App{
		ctx:      ctx,
		cancel:   cancel,
		facade:   f,
		statusCh: f.StatusUpdates(),
		frame:    frame.New("mirabilis", f.Version(), screens.MenuItems()),
		router:   router.New(menu),
	}
}

func startChromeTick(gen int) tea.Cmd {
	if a11y.ReducedMotion() {
		return nil
	}
	return tea.Tick(chromePeriod, func(time.Time) tea.Msg {
		return chromeTickMsg{gen: gen}
	})
}

func (a *App) handleChromeTick(msg chromeTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != a.chromeGen {
		return a, nil
	}
	a.chromeFrame++
	a.frame.SetChrome(a.chromeFrame)
	ct := bus.ChromeTick{Frame: a.chromeFrame}
	a.router, _ = a.router.Update(bus.Envelope{Msg: ct})
	return a, startChromeTick(a.chromeGen)
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.router.Init(),
		watchStatus(a.statusCh),
		startChromeTick(a.chromeGen),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.winW, a.winH = msg.Width, msg.Height
		var fc, rc tea.Cmd
		a.frame, fc = a.frame.Update(msg)
		mw, mh := a.frame.MainSize()
		a.router, rc = a.router.Update(tea.WindowSizeMsg{Width: mw, Height: mh})
		return a, tea.Batch(fc, rc)

	case statusMsg:
		var fc tea.Cmd
		snap := obs.Snapshot(msg)
		sc := bus.StatusChanged{Snapshot: snap}
		a.frame, fc = a.frame.Update(sc)
		return a, tea.Batch(fc, watchStatus(a.statusCh))

	case tea.KeyPressMsg:
		return a.handleKey(msg)

	case busyTickMsg:
		return a.handleBusyTick(msg)

	case chromeTickMsg:
		return a.handleChromeTick(msg)

	case bus.MenuChosen:
		return a.handleMenuChosen(msg)

	case pipelineEventMsg:
		return a.handlePipelineEvent(msg)

	case pipelineDoneMsg:
		return a.handlePipelineDone(msg)

	case bus.ScreenResult:
		return a.handleScreenResult(msg)

	case bus.ScreenPop:
		return a.handleScreenPop()

	case execDoneMsg:
		return a.handleExecDone(msg)

	case resetDoneMsg:
		return a.handleResetDone(msg)

	case telegramDoneMsg:
		return a.handleTelegramDone(msg)

	case harnessStatusMsg:
		return a.handleHarnessStatus(msg)

	case harnessDoneMsg:
		return a.handleHarnessDone(msg)

	case vscodeDoneMsg:
		return a.handleVSCodeDone(msg)

	case bus.CopyRequest:
		return a.handleCopyRequest(msg)

	case openURLDoneMsg:
		return a, nil

	case copyDoneMsg:
		return a.handleCopyDone(msg)
	}

	var cmd tea.Cmd
	a.router, cmd = a.router.Update(msg)
	return a, cmd
}

type mainAreaScreen interface{ MainArea() bool }

func isOverlay(s router.Screen) bool {
	_, ok := s.(mainAreaScreen)
	return !ok
}

func (a App) View() tea.View {
	var content string
	var cx, cy int
	if a.router.Depth() > 1 && a.winW > 0 && a.winH > 0 && isOverlay(a.router.Top()) {
		var bx, by int
		content, bx, by = a.overlayView()
		cx, cy = bx+2, by+1
	} else {
		content = a.frame.View(a.router.View())
		cx, cy = a.frame.MenuCursorCell()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.Cursor = &tea.Cursor{
		Position: tea.Position{X: cx, Y: cy},
		Shape:    tea.CursorBar,
		Blink:    false,
	}
	return v
}

func (a App) overlayView() (content string, boxX, boxY int) {
	base := a.frame.View(a.router.Below().View())
	box := styles.Overlay.Render(a.router.Top().View())

	ox, oy := a.frame.MainOrigin()
	mainW := max(a.winW-ox, 0)
	boxW, boxH := lipgloss.Width(box), lipgloss.Height(box)

	cx := ox + max((mainW-boxW)/2, 0)
	cy := oy + max((a.winH-2-boxH)/2, 0)
	if cx+boxW > a.winW {
		cx = max(a.winW-boxW, 0)
	}
	if cy+boxH > a.winH {
		cy = max(a.winH-boxH, 0)
	}

	canvas := lipgloss.NewCanvas(a.winW, a.winH)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(base).Z(0),
		lipgloss.NewLayer(box).X(cx).Y(cy).Z(1),
	))
	return canvas.Render(), cx, cy
}
