package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

var seedManagedKeys = map[string]bool{
	"hooks":      true,
	"statusLine": true,
	"env":        true,
}

func mergeSettings(dest, seed map[string]any) map[string]any {
	out := make(map[string]any, len(dest))
	for k, v := range dest {
		out[k] = v
	}
	for k, sv := range seed {
		if seedManagedKeys[k] {
			out[k] = sv
			continue
		}
		if dv, ok := out[k]; ok {
			dm, dIsMap := dv.(map[string]any)
			sm, sIsMap := sv.(map[string]any)
			if dIsMap && sIsMap {
				out[k] = mergeSettings(dm, sm)
				continue
			}
		}
		if _, ok := out[k]; !ok {
			out[k] = sv
		}
	}
	return out
}

type settingsStep struct {
	d Deps
}

func (s *settingsStep) Meta() pipeline.Meta { return carryMeta("settings", "Claude settings") }

func (s *settingsStep) Check(_ context.Context) (bool, error) {
	cd := s.d.claudeDir()
	if !exists(cd) || !exists(filepath.Join(cd, "xdg-data")) {
		return false, nil
	}
	seedPath := s.d.Cfg.SettingsSeed()
	if !exists(seedPath) {
		return true, nil
	}
	dest := s.d.settingsPath()
	if !exists(dest) {
		return false, nil
	}
	seed, err := readJSON(seedPath)
	if err != nil {
		return true, nil
	}
	m, err := readJSON(dest)
	if err != nil {
		return true, nil
	}
	for k := range seedManagedKeys {
		if _, inSeed := seed[k]; !inSeed {
			continue
		}
		if _, ok := m[k]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func (s *settingsStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	cd := s.d.claudeDir()
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return fmt.Errorf("mkdir ~/.claude: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cd, "xdg-data"), 0o755); err != nil {
		return fmt.Errorf("mkdir ~/.claude/xdg-data: %w", err)
	}
	seed := s.d.Cfg.SettingsSeed()
	if !exists(seed) {
		return nil
	}
	dest := s.d.settingsPath()
	mu := pathMutex(dest)
	mu.Lock()
	defer mu.Unlock()
	if exists(dest) {
		dm, derr := readJSON(dest)
		sm, serr := readJSON(seed)
		if derr == nil && serr == nil {
			merged := mergeSettings(dm, sm)
			delete(merged, "sandbox")
			if werr := writeJSON(dest, merged); werr == nil {
				return nil
			}
		}
	}
	return copyFile(seed, dest)
}

type effortStep struct {
	d Deps
}

func (s *effortStep) Meta() pipeline.Meta {
	m := carryMeta("effort", "Effort level")
	m.Deps = []string{"settings"}
	return m
}

func (s *effortStep) effort() string {
	if ef, ok := effortFromEnv(); ok {
		return ef
	}
	lo, _ := s.d.loadout()
	return lo.Effort
}

func (s *effortStep) Check(_ context.Context) (bool, error) {
	ef := s.effort()
	if ef == "" {
		return true, nil
	}
	dest := s.d.settingsPath()
	if !exists(dest) {
		return true, nil
	}
	m, err := readJSON(dest)
	if err != nil {
		return true, nil
	}
	return m["effortLevel"] == ef, nil
}

func (s *effortStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	ef := s.effort()
	if ef == "" {
		return nil
	}
	dest := s.d.settingsPath()
	if !exists(dest) {
		return nil
	}
	return updateJSON(dest, func(m map[string]any) error {
		m["effortLevel"] = ef
		return nil
	})
}

type themeStep struct {
	d Deps
}

func (s *themeStep) Meta() pipeline.Meta {
	m := carryMeta("theme", "Claude theme")
	m.Deps = []string{"settings"}
	return m
}

const defaultTheme = "auto"

func (s *themeStep) theme() string {
	data, err := os.ReadFile(s.d.themePath())
	if err != nil {
		return defaultTheme
	}
	if t := strings.TrimRight(string(data), "\r\n"); t != "" {
		return t
	}
	return defaultTheme
}

func (s *themeStep) Check(_ context.Context) (bool, error) {
	th := s.theme()
	dest := s.d.settingsPath()
	if !exists(dest) {
		return true, nil
	}
	m, err := readJSON(dest)
	if err != nil {
		return true, nil
	}
	return m["theme"] == th, nil
}

func (s *themeStep) Run(_ context.Context, _ chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	th := s.theme()
	dest := s.d.settingsPath()
	if !exists(dest) {
		return nil
	}
	return updateJSON(dest, func(m map[string]any) error {
		m["theme"] = th
		return nil
	})
}
