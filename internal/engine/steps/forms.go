package steps

import (
	"context"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const pluginsDisabledKey = "PLUGINS_DISABLED"

type formStep struct {
	persisted func() bool
	load      func() Catalog
	save      func(choice []string) error
	name      string
	title     string
}

func (s *formStep) Meta() pipeline.Meta {
	return pipeline.Meta{Name: s.name, Title: s.title, Kind: pipeline.Interactive}
}

func (s *formStep) Check(context.Context) (bool, error) {
	return s.persisted(), nil
}

func (s *formStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: s.name, Payload: s.load()}
	r, err := awaitResume(ctx, in)
	if err != nil {
		return err
	}
	choice, err := asStrings(s.name, r.Value)
	if err != nil {
		return err
	}
	return s.save(choice)
}

func newStacksForm(d Deps) *formStep {
	return &formStep{
		name:  "stacks",
		title: "Stacks",
		persisted: func() bool {
			_, ok := config.ReadStacks(d.Repo)
			return ok
		},
		load: func() Catalog {
			cur, _ := config.ReadStacks(d.Repo)
			return Catalog{
				Title:       "Optional stacks",
				Options:     config.ReadStackCatalog(d.Repo),
				Selected:    splitCSV(cur),
				MultiSelect: true,
			}
		},
		save: func(choice []string) error {
			return config.WriteStacks(d.Repo, strings.Join(choice, ","))
		},
	}
}

func newPluginsForm(d Deps) *formStep {
	return &formStep{
		name:  "plugins-form",
		title: "Plugins choice",
		persisted: func() bool {
			_, ok := dotenvRead(d.Repo, pluginsDisabledKey)
			return ok
		},
		load: func() Catalog {
			catalog := config.ReadPluginCatalog(d.Repo)
			return Catalog{
				Title:       "Plugins",
				Options:     catalog,
				Selected:    subtract(catalog, config.ReadPluginsDisabled(d.Repo)),
				MultiSelect: true,
			}
		},
		save: func(choice []string) error {
			return config.WritePluginsDisabled(d.Repo, subtract(config.ReadPluginCatalog(d.Repo), choice))
		},
	}
}

func newSkillsForm(d Deps) *formStep {
	return &formStep{
		name:  "skills-form",
		title: "Skills choice",
		persisted: func() bool {
			_, ok := config.ReadSkills(d.Repo)
			return ok
		},
		load: func() Catalog {
			cur, _ := config.ReadSkills(d.Repo)
			return Catalog{
				Title:       "Optional skills",
				Options:     config.ReadSkillCatalog(d.Repo),
				Selected:    splitCSV(cur),
				MultiSelect: true,
			}
		},
		save: func(choice []string) error {
			return config.WriteSkills(d.Repo, strings.Join(choice, ","))
		},
	}
}
