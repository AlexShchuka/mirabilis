package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	huh "charm.land/huh/v2"
)

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func joinCSV(items []string) string { return strings.Join(items, ",") }

func runForm(form *huh.Form) (bool, error) {
	form = form.WithOutput(os.Stderr).WithInput(os.Stdin)
	if err := form.Run(); err != nil {
		return false, err
	}
	return form.State == huh.StateCompleted, nil
}

func RunPlugins(args []string) error {
	fs := flag.NewFlagSet("plugins", flag.ContinueOnError)
	optionsCSV := fs.String("options", "", "")
	selectedCSV := fs.String("selected", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	options := splitCSV(*optionsCSV)
	selected := splitCSV(*selectedCSV)
	if len(options) == 0 {
		return errors.New("plugins: --options is required")
	}

	opts := make([]huh.Option[string], 0, len(options))
	for _, id := range options {
		opts = append(opts, huh.NewOption(id, id).Selected(contains(selected, id)))
	}
	var chosen []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Плагины (пробел — переключить, Enter — ок)").
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

func RunSecrets(_ []string) error {
	var which []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Что настроить (пробел — выбрать, Enter — ок)").
				Options(
					huh.NewOption("context7 API key", "context7"),
					huh.NewOption("Telegram bot token", "telegram-token"),
					huh.NewOption("Telegram chat id", "telegram-chat"),
				).
				Value(&which),
		),
	)
	ok, err := runForm(form)
	if err != nil && !errors.Is(err, huh.ErrUserAborted) {
		return err
	}
	if !ok {
		return nil
	}
	for _, name := range which {
		fmt.Println(name)
	}
	return nil
}

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
				Title("Опциональные стеки (node + python + go уже в базе)").
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
