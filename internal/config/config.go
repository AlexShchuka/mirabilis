package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Base string
}

func New(base string) Config { return Config{Base: base} }

func (c Config) SettingsSeed() string   { return filepath.Join(c.Base, "settings.json") }
func (c Config) HudConfigSeed() string  { return filepath.Join(c.Base, "claude-hud.json") }
func (c Config) RTKConfigSeed() string  { return filepath.Join(c.Base, "rtk-config.toml") }
func (c Config) PluginsTxt() string     { return filepath.Join(c.Base, "plugins.txt") }
func (c Config) AptPackagesTxt() string { return filepath.Join(c.Base, "apt-packages.txt") }

type MemoryCategory struct {
	Name       string
	MemoryType string
	Summary    string
}

var MemoryCategories = []MemoryCategory{
	{"about-me", "semantic", "Stable facts about you: identity, role, goals, hard preferences, constraints."},
	{"workstreams", "semantic", "Active and recurring projects and repos — one-line what + where + status pointers, not full state."},
	{"dev-principles", "procedural", "Cross-project engineering invariants you endorse: style, testing bar, anti-slop."},
	{"sandbox-ops", "procedural", "How to operate this container: tools, boundaries, build and run commands, gotchas."},
	{"domain-knowledge", "semantic", "Durable, reusable facts learned while studying, researching, or reading."},
	{"research-log", "episodic", "Dated findings tied to a specific investigation, paper, or bug. Append-only, compacted periodically."},
	{"prep", "episodic", "Interview-prep and learning state: topics drilled, weak spots, study targets."},
}

func ReadStacks(repo string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(repo, ".env"))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "STACKS="); ok {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

func ReadStackCatalog(repo string) []string {
	data, err := os.ReadFile(filepath.Join(repo, "config", "stacks.txt"))
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

func WriteStacks(repo, csv string) error {
	path := filepath.Join(repo, ".env")
	var keep []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "STACKS=") {
				keep = append(keep, line)
			}
		}
	}
	out := strings.TrimRight(strings.Join(keep, "\n"), "\n")
	if out != "" {
		out += "\n"
	}
	out += "STACKS=" + csv + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}

func ReadPluginCatalog(repo string) []string {
	data, err := os.ReadFile(filepath.Join(repo, "config", "plugins.txt"))
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

func ReadPluginsDisabled(repo string) []string {
	data, err := os.ReadFile(filepath.Join(repo, ".env"))
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "PLUGINS_DISABLED="); ok {
			rest = strings.TrimSpace(rest)
			if rest == "" {
				return nil
			}
			var out []string
			for _, p := range strings.Split(rest, ",") {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
			return out
		}
	}
	return nil
}

func WritePluginsDisabled(repo string, disabled []string) error {
	path := filepath.Join(repo, ".env")
	var keep []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "PLUGINS_DISABLED=") {
				keep = append(keep, line)
			}
		}
	}
	out := strings.TrimRight(strings.Join(keep, "\n"), "\n")
	if out != "" {
		out += "\n"
	}
	out += "PLUGINS_DISABLED=" + strings.Join(disabled, ",") + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}
