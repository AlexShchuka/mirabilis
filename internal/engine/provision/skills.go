package provision

import (
	"context"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/provision/skills"
)

type skillsStep struct {
	d Deps
}

func (s *skillsStep) Meta() pipeline.Meta { return installMeta("skills", "Claude skills") }

func (s *skillsStep) units() []skills.Unit {
	return skills.Units(config.SkillGroupsFrom(s.d.Cfg.SkillsTxt()), s.d.selectedSkills())
}

func (s *skillsStep) installer() skills.Installer {
	return skills.Installer{OK: s.d.argvOK, Output: s.d.output, Stream: s.d.stream}
}

func (s *skillsStep) Check(ctx context.Context) (bool, error) {
	return s.installer().Satisfied(ctx, s.units())
}

func (s *skillsStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	return s.installer().Apply(ctx, out, s.units())
}
