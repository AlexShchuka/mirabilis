package app

import (
	"context"
	"io"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/frame"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	"github.com/AlexShchuka/mirabilis/internal/tui/screens"
)

type Facade interface {
	LaunchSteps() []pipeline.Command
	Logger() *slog.Logger
	StatusUpdates() <-chan obs.Snapshot
	OnTokenExtracted(token string)
	NewTokenTee() (io.Writer, func() (string, bool))
	SaveMemory(ctx context.Context) error
	ResetSandbox(ctx context.Context) error
}

var execRunner = tea.Exec

type App struct {
	ctx        context.Context
	cancel     context.CancelFunc
	facade     Facade
	statusCh   <-chan obs.Snapshot
	frame      frame.Model
	router     router.Model
	pipe       *pipeline.Pipeline
	waiting    string
	menuAction string
	winW       int
	winH       int
}

func New(ctx context.Context, f Facade) App {
	ctx, cancel := context.WithCancel(ctx)
	menu := screens.NewMenu("app/menu")
	a := App{
		ctx:      ctx,
		cancel:   cancel,
		facade:   f,
		statusCh: f.StatusUpdates(),
		frame:    frame.New("mirabilis", "v2.0.0", screens.MenuItems()),
		router:   router.New(menu),
	}
	return a
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
		a.router, rc = a.router.Update(msg)
		return a, tea.Batch(fc, rc)

	case statusMsg:
		var fc tea.Cmd
		sc := bus.StatusChanged{Snapshot: obs.Snapshot(msg)}
		a.frame, fc = a.frame.Update(sc)
		return a, tea.Batch(fc, watchStatus(a.statusCh))

	case tea.KeyPressMsg:
		return a.handleKey(msg)

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
	}

	var cmd tea.Cmd
	a.router, cmd = a.router.Update(msg)
	return a, cmd
}

func (a App) View() tea.View {
	return tea.NewView(a.frame.View(a.router.View()))
}
