package steps

import (
	"context"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	pluginsDisabledFile = ".mirabilis-plugins-disabled"
	skillsFile          = ".mirabilis-skills"
)

type applyStep struct {
	d       Deps
	desired func() []string
	name    string
	title   string
	formDep string
	file    string
	envVar  string
	phase   string
}

func newPluginsApply(d Deps) *applyStep {
	return &applyStep{
		d:       d,
		desired: func() []string { return config.ReadPluginsDisabled(d.Repo) },
		name:    "plugins",
		title:   "Plugins",
		formDep: configStepName,
		file:    pluginsDisabledFile,
		envVar:  "MDIS",
		phase:   "plugins",
	}
}

func newSkillsApply(d Deps) *applyStep {
	return &applyStep{
		d: d,
		desired: func() []string {
			v, _ := config.ReadSkills(d.Repo)
			return splitCSV(v)
		},
		name:    "skills",
		title:   "Skills",
		formDep: configStepName,
		file:    skillsFile,
		envVar:  "MSKILLS",
		phase:   "skills",
	}
}

func (s *applyStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     s.name,
		Title:    s.title,
		Deps:     []string{"provision-start", s.formDep},
		Kind:     pipeline.Auto,
		Optional: true,
		Timeout:  5 * time.Minute,
	}
}

func (s *applyStep) Check(ctx context.Context) (bool, error) {
	out, _ := exec.Run(ctx, s.d.Runner, exec.Spec{
		Argv: containerArgv("bash", "-lc", `cat "$HOME/.claude/`+s.file+`" 2>/dev/null`),
	})
	return setsEqual(splitLines(out), s.desired()), nil
}

func (s *applyStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	content := strings.Join(s.desired(), "\n")
	write := containerArgv("env", s.envVar+"="+content, "bash", "-lc",
		`printf '%s' "$`+s.envVar+`" > "$HOME/.claude/`+s.file+`"`)
	if err := stream(s.name, out, s.d.Runner.Stream(ctx, exec.Spec{Argv: write})); err != nil {
		return err
	}
	apply := containerArgv("mirabilis", "provision", "--phase", s.phase)
	return stream(s.name, out, s.d.Runner.Stream(ctx, exec.Spec{Argv: apply}))
}
