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
	"github.com/AlexShchuka/mirabilis/internal/tui/frame"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
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
	AttachExec(ctx context.Context) (argv, env []string, err error)
	LastHarnessChoice() string
	RememberHarnessChoice(choice string) error
	TelegramConfigured() bool
	MarkTelegramConfigured() error
}

var execRunner = tea.Exec

type App struct {
	ctx             context.Context
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
	harnessChoice   string
	secondary       bool
	baseNotice      string
	errNotice       string
	attachReady     bool
}

func New(ctx context.Context, f Facade, secondary bool) App {
	ctx, cancel := context.WithCancel(ctx)
	menu := screens.NewMenu("app/menu")
	a := App{
		ctx:      ctx,
		cancel:   cancel,
		facade:   f,
		statusCh: f.StatusUpdates(),
		frame:    frame.New("mirabilis", f.Version(), screens.MenuItems()),
		router:   router.New(menu),
	}
	if secondary {
		a.secondary = true
		a.baseNotice = uistr.NoticeSecondary
		a.applySecondary()
		a.router = router.New(menu.WithNotice(a.baseNotice))
	}
	return a
}

func (a *App) applySecondary() {
	for _, action := range []string{screens.ActionLaunch, screens.ActionHarness, screens.ActionTelegram, screens.ActionReset} {
		a.frame.SetEnabled(action, false)
	}
}

func (a *App) promote() {
	a.secondary = false
	a.baseNotice = ""
	for _, action := range []string{screens.ActionLaunch, screens.ActionHarness, screens.ActionTelegram, screens.ActionReset} {
		a.frame.SetEnabled(action, true)
	}
}

func (a *App) applyContainerState(snap obs.Snapshot) {
	running := snap["container"].State == obs.StateOK
	if running != a.attachReady {
		a.attachReady = running
		a.frame.SetEnabled(screens.ActionAttach, running)
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.router.Init(),
		watchStatus(a.statusCh),
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
		a.applyContainerState(snap)
		sc := bus.StatusChanged{Snapshot: snap}
		a.frame, fc = a.frame.Update(sc)
		return a, tea.Batch(fc, watchStatus(a.statusCh))

	case promotedMsg:
		return a.handlePromoted()

	case tea.KeyPressMsg:
		return a.handleKey(msg)

	case busyTickMsg:
		return a.handleBusyTick(msg)

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

	case attachReadyMsg:
		return a.handleAttachReady(msg)
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
	if a.router.Depth() > 1 && a.winW > 0 && a.winH > 0 && isOverlay(a.router.Top()) {
		content = a.overlayView()
	} else {
		content = a.frame.View(a.router.View())
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a App) overlayView() string {
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
	return canvas.Render()
}
