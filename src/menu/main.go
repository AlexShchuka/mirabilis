package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		runMainMenu()
		return
	}
	var err error
	switch args[0] {
	case "plugins":
		err = RunPlugins(args[1:])
	case "secrets":
		err = RunSecrets(args[1:])
	case "theme":
		err = RunTheme(args[1:])
	case "stacks":
		err = RunStacks(args[1:])
	case "harness":
		err = RunHarness(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "mirabilis-menu: unknown subcommand %q\n", args[0])
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mirabilis-menu: %v\n", err)
		os.Exit(1)
	}
}

func runMainMenu() {
	st := FromStdin()
	m := New(st)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mirabilis-menu: %v\n", err)
		os.Exit(1)
	}
	action := Action(final)
	if action == "" {
		action = "quit"
	}
	fmt.Println(action)
}
