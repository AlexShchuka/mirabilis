package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/src/menu/internal/forms"
	"github.com/AlexShchuka/mirabilis/src/menu/internal/mainmenu"
	"github.com/AlexShchuka/mirabilis/src/menu/internal/status"
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
		err = forms.RunPlugins(args[1:])
	case "secrets":
		err = forms.RunSecrets(args[1:])
	case "theme":
		err = forms.RunTheme(args[1:])
	case "stacks":
		err = forms.RunStacks(args[1:])
	case "harness":
		err = forms.RunHarness(args[1:])
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
	st := status.FromStdin()
	m := mainmenu.New(st)
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	final, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mirabilis-menu: %v\n", err)
		os.Exit(1)
	}
	action := mainmenu.Action(final)
	if action == "" {
		action = "quit"
	}
	fmt.Println(action)
}
