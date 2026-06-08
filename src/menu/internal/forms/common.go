package forms

import (
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
