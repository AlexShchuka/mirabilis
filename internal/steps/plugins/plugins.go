package plugins

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/steps"
)

type step struct{}

func (step) Check(ctx context.Context, r runner.Runner) (bool, error) {
	raw, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-plugins-disabled" 2>/dev/null`)
	containerDisabled := splitLines(raw)
	hostDisabled := config.ReadPluginsDisabled(r.Repo())
	return setsEqual(containerDisabled, hostDisabled), nil
}

func (step) Run(ctx context.Context, r runner.Runner) error {
	disabled := config.ReadPluginsDisabled(r.Repo())
	content := strings.Join(disabled, "\n")
	if _, err := r.Container(ctx, "env", "MDIS="+content,
		"bash", "-lc", `printf '%s' "$MDIS" > "$HOME/.claude/.mirabilis-plugins-disabled"`); err != nil {
		return err
	}
	_, err := r.Container(ctx, "mirabilis", "provision", "--phase", "plugins")
	return err
}

func init() {
	steps.Register(pipeline.StepMeta{
		Name:     "plugins",
		Title:    "Plugins",
		Detail:   "applying plugin selection",
		Deps:     []string{"prepare"},
		Retry:    pipeline.RetryNet,
		Optional: true,
		Timeout:  180 * time.Second,
	}, step{})
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func setsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ca := make([]string, len(a))
	cb := make([]string, len(b))
	copy(ca, a)
	copy(cb, b)
	slices.Sort(ca)
	slices.Sort(cb)
	for i := range ca {
		if ca[i] != cb[i] {
			return false
		}
	}
	return true
}
