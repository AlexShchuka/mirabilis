package forms

import (
	"errors"
	"flag"
	"fmt"

	huh "charm.land/huh/v2"
)

func RunStacks(args []string) error {
	fs := flag.NewFlagSet("stacks", flag.ContinueOnError)
	optionsCSV := fs.String("options", "", "")
	selectedCSV := fs.String("selected", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	options := splitCSV(*optionsCSV)
	selected := splitCSV(*selectedCSV)
	if len(options) == 0 {
		return errors.New("stacks: --options is required")
	}
	opts := make([]huh.Option[string], 0, len(options))
	for _, id := range options {
		opts = append(opts, huh.NewOption(id, id).Selected(contains(selected, id)))
	}
	var chosen []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Опциональные стеки (node + python уже в базе)").
				Options(opts...).
				Value(&chosen),
		),
	)
	ok, err := runForm(form)
	if err != nil && !errors.Is(err, huh.ErrUserAborted) {
		return err
	}
	if !ok {
		fmt.Println(joinCSV(selected))
		return nil
	}
	fmt.Println(joinCSV(chosen))
	return nil
}
