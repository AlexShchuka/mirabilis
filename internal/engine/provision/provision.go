package provision

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	UpstreamFileName  = ".mirabilis-headroom-upstream"
	settingsEnvKey    = "env"
	settingsURLKey    = "ANTHROPIC_BASE_URL"
	settingsTokenKey  = "ANTHROPIC_AUTH_TOKEN"
	headroomVenvRel   = ".headroom-venv/bin/headroom"
	headroomPollLimit = 60
)

var (
	settingsBaseURL  = config.HeadroomBaseURL()
	headroomStatsURL = config.HeadroomStatsURL()
)

type Deps struct {
	Runner      exec.Runner
	Cfg         config.Config
	Log         *slog.Logger
	Repo        string
	Home        string
	ProxyAddr   string
	SessionKey  string
	Fingerprint string
}

func (d Deps) claudeDir() string    { return filepath.Join(d.Home, ".claude") }
func (d Deps) settingsPath() string { return filepath.Join(d.claudeDir(), "settings.json") }
func (d Deps) upstreamPath() string { return filepath.Join(d.claudeDir(), UpstreamFileName) }
func (d Deps) headroomBin() string  { return filepath.Join(d.Home, headroomVenvRel) }

func Create(d Deps) []pipeline.Command {
	return append(carryCreate(d), &createMarkerStep{d: d})
}

func Start(d Deps) []pipeline.Command {
	out := []pipeline.Command{&credentialsStep{d: d}, &headroomStep{d: d}, &settingsEnvStep{d: d}}
	out = append(out, carryStart(d)...)
	return append(out, &startMarkerStep{d: d})
}

func Plugins(d Deps) []pipeline.Command { return carryPlugins(d) }

func Skills(d Deps) []pipeline.Command { return carrySkills(d) }

func RunPhase(ctx context.Context, d Deps, phase string) error {
	var steps []pipeline.Command
	switch phase {
	case "create":
		steps = Create(d)
	case "start":
		steps = Start(d)
	case "plugins":
		steps = Plugins(d)
	case "skills":
		steps = Skills(d)
	default:
		return fmt.Errorf("provision: unknown phase %q", phase)
	}
	log := d.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	p, err := pipeline.New(log, steps...)
	if err != nil {
		return err
	}
	go streamToStdout(p.Events())
	return p.Run(ctx)
}
