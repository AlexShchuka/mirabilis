// Package provision implements the idempotent provisioning steps run inside the dev-container.
package provision

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	fileHarness         = ".mirabilis-harness"
	filePluginsDisabled = ".mirabilis-plugins-disabled"
	fileTheme           = ".mirabilis-theme"
	fileSkills          = ".mirabilis-skills"
	fileLoadout         = ".mirabilis-loadout"
	harnessSkip         = "skip"
	harnessInstall      = "install"
	carryTimeout        = 5 * time.Minute
	installTimeout      = 15 * time.Minute
)

func carryCreate(d Deps) []pipeline.Command {
	return []pipeline.Command{
		&loadoutStep{d: d},
		&settingsStep{d: d},
		&onboardingStep{d: d},
		&themeStep{d: d},
		&effortStep{d: d},
		&memoryStep{d: d},
		&gitIdentityStep{d: d},
		&hudStep{d: d},
		&mcpStep{d: d},
		&caveShrinkStep{d: d},
		&rtkStep{d: d},
		&rtkConfigStep{d: d},
	}
}

func carryStart(d Deps) []pipeline.Command {
	return []pipeline.Command{
		&ecosystemStep{d: d},
		&harnessStep{d: d},
		&pluginsStep{d: d},
		&skillsStep{d: d},
		&mathToolsStep{d: d},
	}
}

func carryPlugins(d Deps) []pipeline.Command { return []pipeline.Command{&pluginsStep{d: d}} }

func carrySkills(d Deps) []pipeline.Command { return []pipeline.Command{&skillsStep{d: d}} }

func carryMeta(name, title string) pipeline.Meta {
	return pipeline.Meta{
		Name:     name,
		Title:    title,
		Optional: true,
		Kind:     pipeline.Auto,
		Parallel: true,
		Timeout:  carryTimeout,
	}
}

func installMeta(name, title string) pipeline.Meta {
	m := carryMeta(name, title)
	m.Timeout = installTimeout
	return m
}

func (d Deps) output(ctx context.Context, argv ...string) (string, error) {
	return exec.Run(ctx, d.Runner, exec.Spec{Argv: argv})
}

func (d Deps) argvOK(ctx context.Context, argv ...string) bool {
	_, err := d.output(ctx, argv...)
	return err == nil
}

func (d Deps) scriptOK(ctx context.Context, script string) bool {
	return d.argvOK(ctx, "bash", "-lc", script)
}

func (d Deps) stream(ctx context.Context, step string, out chan<- pipeline.Event, argv ...string) error {
	for ev := range d.Runner.Stream(ctx, exec.Spec{Argv: argv}) {
		pipeline.Forward(step, out, ev)
		if ev.Kind == exec.KindExited {
			return ev.Err
		}
	}
	return nil
}

func (d Deps) streamScript(ctx context.Context, step string, out chan<- pipeline.Event, script string) error {
	return d.stream(ctx, step, out, "bash", "-lc", script)
}

func (d Deps) loadout() (config.Loadout, bool) {
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), fileLoadout))
	name := config.DefaultLoadout
	if err == nil {
		if s := strings.TrimSpace(string(data)); s != "" {
			name = s
		}
	}
	if lo, ok := config.ReadLoadoutManifest(d.Repo, name); ok {
		return lo, true
	}
	if lo, ok := config.ReadLoadoutManifest(d.Repo, config.DefaultLoadout); ok {
		return lo, true
	}
	return config.Loadout{}, false
}

func (d Deps) harnessChoice() string {
	if lo, ok := d.loadout(); ok && !lo.Harness {
		return harnessSkip
	}
	data, err := os.ReadFile(filepath.Join(d.claudeDir(), fileHarness))
	if err != nil {
		return harnessInstall
	}
	return strings.TrimSpace(string(data))
}

func (d Deps) themePath() string { return filepath.Join(d.claudeDir(), fileTheme) }

func (d Deps) disabledPlugins() map[string]bool {
	explicit := readSet(filepath.Join(d.claudeDir(), filePluginsDisabled))
	lo, ok := d.loadout()
	if !ok || len(lo.Plugins) == 0 {
		return explicit
	}
	allowed := make(map[string]bool, len(lo.Plugins))
	for _, p := range lo.Plugins {
		allowed[p] = true
	}
	all := config.ReadPluginCatalog(d.Repo)
	out := make(map[string]bool)
	for _, p := range all {
		if !allowed[p] {
			out[p] = true
		}
	}
	for p := range explicit {
		out[p] = true
	}
	return out
}

func (d Deps) selectedSkills() map[string]bool {
	return readSet(filepath.Join(d.claudeDir(), fileSkills))
}

func readSet(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out[line] = true
		}
	}
	return out
}

func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type loadoutStep struct {
	d Deps
}

func (s *loadoutStep) Meta() pipeline.Meta { return carryMeta("loadout", "Sandbox loadout") }

func (s *loadoutStep) desired() string {
	name, ok := config.ReadLoadout(s.d.Repo)
	if !ok || name == "" {
		name = config.DefaultLoadout
	}
	return name
}

func (s *loadoutStep) Check(_ context.Context) (bool, error) {
	data, err := os.ReadFile(filepath.Join(s.d.claudeDir(), fileLoadout))
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(data)) == s.desired(), nil
}

func (s *loadoutStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	dir := s.d.claudeDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, fileLoadout), []byte(s.desired()+"\n"), 0o644); err != nil {
		return err
	}
	lo, ok := s.d.loadout()
	hpref := harnessInstall
	if ok && !lo.Harness {
		hpref = harnessSkip
	}
	return os.WriteFile(filepath.Join(dir, fileHarness), []byte(hpref+"\n"), 0o644)
}
