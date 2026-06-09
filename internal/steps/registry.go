package steps

import (
	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/steps/auth"
	"github.com/AlexShchuka/mirabilis/internal/steps/claude"
	"github.com/AlexShchuka/mirabilis/internal/steps/container"
	"github.com/AlexShchuka/mirabilis/internal/steps/harness"
	"github.com/AlexShchuka/mirabilis/internal/steps/plugins"
	"github.com/AlexShchuka/mirabilis/internal/steps/preflight"
)

func BuildSteps() []pipeline.Registered {
	var out []pipeline.Registered
	out = append(out, container.Steps()...)
	out = append(out, claude.Steps()...)
	out = append(out, harness.Steps()...)
	out = append(out, auth.Steps()...)
	out = append(out, plugins.Steps()...)
	out = append(out, preflight.Steps()...)
	return out
}
