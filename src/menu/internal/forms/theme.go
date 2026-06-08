package forms

import (
	"errors"
	"flag"
	"fmt"

	huh "charm.land/huh/v2"
)

func RunTheme(args []string) error {
	fs := flag.NewFlagSet("theme", flag.ContinueOnError)
	current := fs.String("current", "auto", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	choice := *current
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Тема").
				Options(
					huh.NewOption("auto", "auto"),
					huh.NewOption("dark", "dark"),
					huh.NewOption("light", "light"),
					huh.NewOption("dark-daltonized", "dark-daltonized"),
					huh.NewOption("light-daltonized", "light-daltonized"),
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
