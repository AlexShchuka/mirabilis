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

type skillsStep struct{}

func (skillsStep) Check(ctx context.Context, r runner.Runner) (bool, error) {
	raw := provision.ReadSkillsContainer(ctx, r)
	containerSkills := splitLines(raw)
	hostSkills := splitCSV(func() string { v, _ := config.ReadSkills(r.Repo()); return v }())
	return setsEqual(containerSkills, hostSkills), nil
}

func (skillsStep) Run(ctx context.Context, r runner.Runner) error {
	skills := splitCSV(func() string { v, _ := config.ReadSkills(r.Repo()); return v }())
	content := strings.Join(skills, "\n")
	if err := provision.WriteSkillsContainer(ctx, r, content); err != nil {
		return err
	}
	_, err := r.Container(ctx, "mirabilis", "provision", "--phase", "skills")
	return err
}

func skillsSteps() []pipeline.Registered {
	return []pipeline.Registered{
		{
			Meta: pipeline.StepMeta{
				Name:     "skills",
				Title:    "Skills",
				Detail:   "applying skill selection",
				Deps:     []string{"prepare"},
				Retry:    pipeline.RetryNet,
				Optional: true,
				Timeout:  180 * time.Second,
			},
			Impl: skillsStep{},
		},
	}
}
