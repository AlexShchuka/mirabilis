package steps

import (
	"context"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

type pluginsStep struct{}

func (pluginsStep) Check(ctx context.Context, r runner.Runner) (bool, error) {
	raw := provision.ReadDisabledPluginsContainer(ctx, r)
	containerDisabled := splitLines(raw)
	hostDisabled := config.ReadPluginsDisabled(r.Repo())
	return setsEqual(containerDisabled, hostDisabled), nil
}

func (pluginsStep) Run(ctx context.Context, r runner.Runner) error {
	disabled := config.ReadPluginsDisabled(r.Repo())
	content := strings.Join(disabled, "\n")
	if err := provision.WriteDisabledPluginsContainer(ctx, r, content); err != nil {
		return err
	}
	_, err := r.Container(ctx, "mirabilis", "provision", "--phase", "plugins")
	return err
}

func pluginsSteps() []pipeline.Registered {
	return []pipeline.Registered{
		{
			Meta: pipeline.StepMeta{
				Name:     "plugins",
				Title:    "Plugins",
				Detail:   "applying plugin selection",
				Deps:     []string{"prepare"},
				Retry:    pipeline.RetryNet,
				Optional: true,
				Timeout:  180 * time.Second,
			},
			Impl: pluginsStep{},
		},
	}
}
