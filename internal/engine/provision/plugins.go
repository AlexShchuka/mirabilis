package provision

import (
	"context"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/provision/plugins"
)

type pluginsStep struct {
	d Deps
}

func (s *pluginsStep) Meta() pipeline.Meta { return installMeta("plugins", "Claude plugins") }

func (s *pluginsStep) plan() plugins.Plan {
	return plugins.BuildPlan(
		readLines(s.d.Cfg.PluginsTxt()),
		s.d.disabledPlugins(),
		s.d.harnessChoice() == harnessSkip,
		config.ReadMarketplaces(s.d.Repo),
	)
}

func (s *pluginsStep) installer() plugins.Installer {
	dest := s.d.settingsPath()
	return plugins.Installer{
		ScriptOK: s.d.scriptOK,
		Output:   s.d.output,
		Stream:   s.d.stream,
		Script:   s.d.streamScript,
		Settings: plugins.SettingsIO{
			Read:   func() (map[string]any, error) { return readJSON(dest) },
			Write:  func(m map[string]any) error { return writeJSON(dest, m) },
			Update: func(mutate func(map[string]any) error) error { return updateJSON(dest, mutate) },
			Exists: func() bool { return exists(dest) },
		},
	}
}

func (s *pluginsStep) Check(ctx context.Context) (bool, error) {
	return s.installer().Satisfied(ctx, s.plan())
}

func (s *pluginsStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	return s.installer().Apply(ctx, out, s.plan())
}
