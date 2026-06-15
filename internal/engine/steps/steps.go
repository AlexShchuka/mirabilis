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

const autoBatchName = "auto-batch"

func Launch(d Deps) []pipeline.Command {
	autoBatch := newBatch(autoBatchName, "Setup", []string{"claude-auth", configStepName}, []pipeline.Command{
		&imageStep{d: d},
		newContainer(d),
		newProvision(d, phaseCreate),
		newProvision(d, phaseStart),
		newPluginsApply(d),
		newSkillsApply(d),
		&harnessStep{d: d},
	})
	return []pipeline.Command{
		newPreflight(d),
		newConfig(d),
		newTelegram(d),
		&claudeAuthStep{d: d},
		autoBatch,
		&ghAuthStep{d: d, deps: []string{autoBatchName}},
		&attachStep{d: d, deps: []string{"claude-auth", autoBatchName}},
	}
}
