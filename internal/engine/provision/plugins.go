package provision

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type pluginsStep struct {
	d Deps
}

func (s *pluginsStep) Meta() pipeline.Meta { return installMeta("plugins", "Claude plugins") }

func pluginBase(p string) string {
	if i := strings.Index(p, "@"); i >= 0 {
		return p[:i]
	}
	return p
}

func (s *pluginsStep) expectedEnabled() map[string]any {
	disabled := s.d.disabledPlugins()
	enabled := make(map[string]any)
	if s.d.harnessChoice() != harnessSkip {
		enabled["neuro-matrix@neuro-matrix"] = true
	}
	for _, p := range readLines(s.d.Cfg.PluginsTxt()) {
		if !disabled[p] {
			enabled[p] = true
		}
	}
	return enabled
}

func (s *pluginsStep) Check(ctx context.Context) (bool, error) {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return true, nil
	}
	catalog := readLines(s.d.Cfg.PluginsTxt())
	if len(catalog) == 0 {
		return true, nil
	}
	disabled := s.d.disabledPlugins()
	listed, _ := s.d.output(ctx, "claude", "plugin", "list")
	for _, p := range catalog {
		if disabled[p] {
			continue
		}
		if !strings.Contains(listed, pluginBase(p)) {
			return false, nil
		}
	}
	dest := s.d.settingsPath()
	if !exists(dest) {
		return true, nil
	}
	m, err := readJSON(dest)
	if err != nil {
		return true, nil
	}
	enabled, _ := m["enabledPlugins"].(map[string]any)
	return reflect.DeepEqual(enabled, s.expectedEnabled()), nil
}

func (s *pluginsStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return nil
	}
	catalog := readLines(s.d.Cfg.PluginsTxt())
	if len(catalog) == 0 {
		return nil
	}
	_ = s.d.streamScript(ctx, "plugins", out, `mkdir -p "$HOME/.cache/tmp"`)
	var errs []error
	for _, market := range config.ReadMarketplaces(s.d.Repo) {
		if err := s.d.stream(ctx, "plugins", out, "claude", "plugin", "marketplace", "add", market); err != nil {
			errs = append(errs, fmt.Errorf("marketplace add %s: %w", market, err))
		}
	}
	disabled := s.d.disabledPlugins()
	listed, _ := s.d.output(ctx, "claude", "plugin", "list")
	for _, p := range catalog {
		if disabled[p] {
			continue
		}
		if strings.Contains(listed, pluginBase(p)) {
			continue
		}
		script := fmt.Sprintf(`TMPDIR="$HOME/.cache/tmp" claude plugin install %q --scope user`, p)
		if err := s.d.streamScript(ctx, "plugins", out, script); err != nil {
			errs = append(errs, fmt.Errorf("plugin install %s: %w", p, err))
		}
	}
	errs = append(errs, s.writeEnabled())
	return errors.Join(errs...)
}

func (s *pluginsStep) writeEnabled() error {
	dest := s.d.settingsPath()
	if !exists(dest) {
		return nil
	}
	m, err := readJSON(dest)
	if err != nil {
		return nil
	}
	m["enabledPlugins"] = s.expectedEnabled()
	return writeJSON(dest, m)
}
