package steps

import (
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
)

func BuildSteps() []pipeline.Registered {
	var out []pipeline.Registered
	out = append(out, containerSteps()...)
	out = append(out, claudeSteps()...)
	out = append(out, harnessSteps()...)
	out = append(out, authSteps()...)
	out = append(out, pluginsSteps()...)
	out = append(out, skillsSteps()...)
	out = append(out, preflightSteps()...)
	return out
}
