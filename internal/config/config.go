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
func (c Config) PluginsTxt() string     { return filepath.Join(c.Base, "plugins.txt") }
func (c Config) AptPackagesTxt() string { return filepath.Join(c.Base, "apt-packages.txt") }
func (c Config) MemoryRulesDir() string { return filepath.Join(c.Base, "memory", "rules") }

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
