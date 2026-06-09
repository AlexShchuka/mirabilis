package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mirabilis: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	r := newExecRunner()
	if err := ensureDocker(ctx); err != nil {
		return err
	}
	st := computeStatus(ctx, r)
	final, err := tea.NewProgram(newApp(ctx, r, st), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return err
	}
	if app, ok := final.(appModel); ok && app.handoff {
		return handoff(r)
	}
	return nil
}
