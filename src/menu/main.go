package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	if len(os.Args) > 1 {

		if os.Args[1] == "stacks" {
			if err := RunStacks(); err != nil {
				fmt.Fprintf(os.Stderr, "mirabilis: %v\n", err)
				os.Exit(1)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "mirabilis: unknown argument %q — just run 'mirabilis'\n", os.Args[1])
		os.Exit(2)
	}
	if err := runMenu(); err != nil {
		fmt.Fprintf(os.Stderr, "mirabilis: %v\n", err)
		os.Exit(1)
	}
}

func runMenu() error {
	ctx := context.Background()
	r := newExecRunner()
	if err := ensureDocker(ctx); err != nil {
		return err
	}
	for {
		st := computeStatus(ctx, r)
		final, err := tea.NewProgram(New(st), tea.WithOutput(os.Stderr)).Run()
		if err != nil {
			return err
		}
		var actErr error
		switch Action(final) {
		case "launch":
			return RunPipeline()
		case "plugins":
			actErr = doPlugins(ctx, r)
		case "harness":
			actErr = doHarness(ctx, r)
		case "stacks":
			actErr = RunStacks()
		case "vscode":
			actErr = doVSCode(ctx, r)
		default:
			return nil
		}
		if actErr != nil {
			fmt.Fprintf(os.Stderr, "mirabilis: %v\n", actErr)
		}
	}
}
