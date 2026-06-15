package steps

import (
	"github.com/AlexShchuka/mirabilis/internal/engine/authproxy"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

type Deps struct {
	Runner     exec.Runner
	Docker     sandbox.Docker
	Sandbox    *sandbox.Sandbox
	Store      secrets.Store
	Tokens     authproxy.TokenSource
	Obs        *obs.Obs
	ProxyAddr  func() string
	SessionKey func() string
	Repo       string
}

type Catalog struct {
	Key         string
	Title       string
	Description string
	Options     []string
	Selected    []string
	MultiSelect bool
}

type Wizard struct {
	Groups []Catalog
}

type WizardResult struct {
	Choices map[string][]string
}

type TelegramSetup struct{}

type GHAuth struct {
	Code string
	URL  string
}

func Launch(d Deps) []pipeline.Command {
	return []pipeline.Command{
		newPreflight(d),
		&claudeAuthStep{d: d},
		newConfig(d),
		newTelegram(d),
		newPullBuild(d),
		newPullRuntime(d),
		&imageStep{d: d},
		newContainer(d),
		newProvision(d, phaseCreate),
		newProvision(d, phaseStart),
		&ghAuthStep{d: d},
		newPluginsApply(d),
		newSkillsApply(d),
		&harnessStep{d: d},
		&attachStep{d: d},
	}
}
