package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	pluginsDisabledKey = "PLUGINS_DISABLED"

	configStepName = "config"
	keyStacks      = "stacks"
	keyPlugins     = "plugins"
	keySkills      = "skills"
)

type formGroup struct {
	persisted func() bool
	load      func() Catalog
	save      func(choice []string) error
	key       string
}

type configStep struct {
	groups []formGroup
}

func newConfig(d Deps) *configStep {
	return &configStep{groups: []formGroup{
		stacksGroup(d),
		pluginsGroup(d),
		skillsGroup(d),
	}}
}

func (s *configStep) Meta() pipeline.Meta {
	return pipeline.Meta{Name: configStepName, Title: "Config", Kind: pipeline.Interactive}
}

func (s *configStep) Check(context.Context) (bool, error) {
	for _, g := range s.groups {
		if !g.persisted() {
			return false, nil
		}
	}
	return true, nil
}

func (s *configStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	cats := make([]Catalog, 0, len(s.groups))
	saves := make(map[string]func([]string) error, len(s.groups))
	for _, g := range s.groups {
		cat := g.load()
		if len(cat.Options) == 0 {
			if err := g.save(nil); err != nil {
				return err
			}
			continue
		}
		cat.Key = g.key
		cats = append(cats, cat)
		saves[g.key] = g.save
	}
	if len(cats) == 0 {
		return nil
	}
	out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: configStepName, Payload: Wizard{Groups: cats}}
	r, err := awaitResume(ctx, in)
	if err != nil {
		return err
	}
	res, ok := r.Value.(WizardResult)
	if !ok {
		return fmt.Errorf("steps: %s: expected WizardResult, got %T", configStepName, r.Value)
	}
	for _, cat := range cats {
		choice, present := res.Choices[cat.Key]
		if !present {
			return fmt.Errorf("steps: %s: result missing group %q", configStepName, cat.Key)
		}
		if err := saves[cat.Key](choice); err != nil {
			return err
		}
	}
	return nil
}

func stacksGroup(d Deps) formGroup {
	return formGroup{
		key: keyStacks,
		persisted: func() bool {
			_, ok := config.ReadStacks(d.Repo)
			return ok
		},
		load: func() Catalog {
			cur, _ := config.ReadStacks(d.Repo)
			return Catalog{
				Title:       "Optional stacks",
				Description: "Docker stacks to activate. Space toggles, Enter confirms.",
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

func pluginsGroup(d Deps) formGroup {
	return formGroup{
		key: keyPlugins,
		persisted: func() bool {
			_, ok := dotenvRead(d.Repo, pluginsDisabledKey)
			return ok
		},
		load: func() Catalog {
			catalog := config.ReadPluginCatalog(d.Repo)
			return Catalog{
				Title:       "Plugins",
				Description: "All enabled by default. Uncheck to disable.",
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

func skillsGroup(d Deps) formGroup {
	return formGroup{
		key: keySkills,
		persisted: func() bool {
			_, ok := config.ReadSkills(d.Repo)
			return ok
		},
		load: func() Catalog {
			cur, _ := config.ReadSkills(d.Repo)
			return Catalog{
				Title:       "Optional skills",
				Description: "Claude skills to load. Space toggles, Enter confirms.",
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
