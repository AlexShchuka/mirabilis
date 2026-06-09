package app

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
)

func Run(ctx context.Context) error {
	r := runtime.NewExecRunner()
	if err := runtime.EnsureDocker(ctx); err != nil {
		return err
	}
	st := provision.ComputeStatus(ctx, r)
	final, err := tea.NewProgram(newApp(ctx, r, st), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return err
	}
	if app, ok := final.(appModel); ok && app.handoff {
		return runtime.Handoff(r)
	}
	return nil
}
