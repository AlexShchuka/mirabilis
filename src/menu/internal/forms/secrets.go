package forms

import (
	"errors"
	"fmt"

	huh "charm.land/huh/v2"
)

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
