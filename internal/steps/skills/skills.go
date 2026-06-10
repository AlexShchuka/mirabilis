package skills

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

type step struct{}

func (step) Check(ctx context.Context, r runner.Runner) (bool, error) {
	raw := provision.ReadSkillsContainer(ctx, r)
	containerSkills := splitLines(raw)
	hostSkills := splitCSV(func() string { v, _ := config.ReadSkills(r.Repo()); return v }())
	return setsEqual(containerSkills, hostSkills), nil
}

func (step) Run(ctx context.Context, r runner.Runner) error {
	skills := splitCSV(func() string { v, _ := config.ReadSkills(r.Repo()); return v }())
	content := strings.Join(skills, "\n")
	if err := provision.WriteSkillsContainer(ctx, r, content); err != nil {
		return err
	}
	_, err := r.Container(ctx, "mirabilis", "provision", "--phase", "skills")
	return err
}

func Steps() []pipeline.Registered {
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
			Impl: step{},
		},
	}
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

func splitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
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
