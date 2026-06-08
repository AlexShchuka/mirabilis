package forms

import (
	"errors"
	"flag"
	"fmt"

	huh "charm.land/huh/v2"
)

func RunHarness(args []string) error {
	fs := flag.NewFlagSet("harness", flag.ContinueOnError)
	current := fs.String("current", "off", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	choice := *current
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("neuro-matrix харнес").
				Options(
					huh.NewOption("Включить", "on"),
					huh.NewOption("Выключить", "off"),
					huh.NewOption("Переустановить", "reinstall"),
				).
				Value(&choice),
		),
	)
	ok, err := runForm(form)
	if err != nil && !errors.Is(err, huh.ErrUserAborted) {
		return err
	}
	if !ok {
		fmt.Println(*current)
		return nil
	}
	fmt.Println(choice)
	return nil
}
