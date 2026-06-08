package main

import (
	"errors"
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

func RunStacks() error {
	repo := repoRoot()
	options := readStackCatalog(repo)
	if len(options) == 0 {
		return errors.New("stacks: no catalog at config/stacks.txt")
	}
	current, _ := readStacks(repo)
	selected := splitCSV(current)
	opts := make([]huh.Option[string], 0, len(options))
	for _, id := range options {
		opts = append(opts, huh.NewOption(id, id).Selected(contains(selected, id)))
	}
	var chosen []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Опциональные стеки (node + python + go уже в базе)").
			Options(opts...).Value(&chosen)))
	ok, err := runForm(form)
	if err != nil && !errors.Is(err, huh.ErrUserAborted) {
		return err
	}
	if !ok {
		return nil
	}
	return writeStacks(repo, joinCSV(chosen))
}
