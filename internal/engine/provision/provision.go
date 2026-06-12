package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	HeadroomPort      = 8787
	UpstreamFileName  = ".mirabilis-headroom-upstream"
	CreateMarkerName  = ".mirabilis-provision-status"
	StartMarkerName   = ".mirabilis-start-marker"
	createMarkerOK    = "ok"
	settingsBaseURL   = "http://127.0.0.1:8787"
	settingsEnvKey    = "env"
	settingsURLKey    = "ANTHROPIC_BASE_URL"
	settingsTokenKey  = "ANTHROPIC_AUTH_TOKEN"
	headroomVenvRel   = ".headroom-venv/bin/headroom"
	headroomStatsURL  = "http://127.0.0.1:8787/stats"
	headroomPollLimit = 60
)

type Deps struct {
	Runner     exec.Runner
	Cfg        config.Config
	Log        *slog.Logger
	Repo       string
	Home       string
	ProxyAddr  string
	SessionKey string
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
	go func() {
		for ev := range p.Events() {
			switch ev.Kind {
			case pipeline.EvSpawn:
				fmt.Fprintf(os.Stdout, "+ %v\n", ev.Argv)
			case pipeline.EvLine:
				fmt.Fprintln(os.Stdout, ev.Line)
			case pipeline.EvFailed:
				fmt.Fprintf(os.Stdout, "[provision] FAIL %s: %v\n", ev.Step, ev.Err)
			case pipeline.EvSkipped:
				if ev.Err != nil {
					fmt.Fprintf(os.Stdout, "[provision] WARN %s: %v\n", ev.Step, ev.Err)
				}
			}
		}
	}()
	return p.Run(ctx)
}

func readJSON(path string) (map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var m map[string]any
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeJSON(path string, m map[string]any) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
