// Package config reads and writes per-repo configuration from config/ files and .env.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	HeadroomPort         = 8787
	defaultAuthProxyPort = 8788

	DeliveredRetention = 10 * time.Minute

	LocalLLMBaseURL   = "http://host.docker.internal:1234/v1"
	LocalLLMModel     = "local-model"
	LocalLLMTimeout   = 60 * time.Second
	LocalLLMMaxTokens = 2048
)

type Config struct {
	Base string
}

func New(base string) Config { return Config{Base: base} }

func (c Config) SettingsSeed() string  { return filepath.Join(c.Base, "settings.json") }
func (c Config) HudConfigSeed() string { return filepath.Join(c.Base, "claude-hud.json") }
func (c Config) RTKConfigSeed() string { return filepath.Join(c.Base, "rtk-config.toml") }
func (c Config) PluginsTxt() string    { return filepath.Join(c.Base, "plugins.txt") }
func (c Config) SkillsTxt() string     { return filepath.Join(c.Base, "skills.txt") }

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
	{"shared-codebook", "semantic", "Bilaterally agreed term mappings: term → definition — example — external anchor. Hard cap ~20 entries; the real boundary is kernel-reducibility (Common Code v3)."},
}

func ReadStacks(repo string) (string, bool) { return envRead(repo, "STACKS") }

func WriteStacks(repo, csv string) error { return envWrite(repo, "STACKS", csv) }

func ReadLastHarness(repo string) (string, bool) { return envRead(repo, "LAST_HARNESS") }

func WriteLastHarness(repo, val string) error { return envWrite(repo, "LAST_HARNESS", val) }

func TelegramConfigured(repo string) bool {
	v, _ := envRead(repo, "TELEGRAM_CONFIGURED")
	return v == "1"
}

func WriteTelegramConfigured(repo string, on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return envWrite(repo, "TELEGRAM_CONFIGURED", v)
}

func ReadSkills(repo string) (string, bool) { return envRead(repo, "SKILLS") }

func WriteSkills(repo, csv string) error { return envWrite(repo, "SKILLS", csv) }

func ReadPluginsDisabled(repo string) []string {
	v, ok := envRead(repo, "PLUGINS_DISABLED")
	if !ok || v == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func WritePluginsDisabled(repo string, disabled []string) error {
	return envWrite(repo, "PLUGINS_DISABLED", strings.Join(disabled, ","))
}

func ReadStackCatalog(repo string) []string {
	return readList(filepath.Join(repo, "config", "stacks.txt"))
}

func ReadPluginCatalog(repo string) []string {
	return readList(filepath.Join(repo, "config", "plugins.txt"))
}

func ReadSkillCatalog(repo string) []string {
	return readList(filepath.Join(repo, "config", "skills.txt"))
}

func ReadMarketplaces(repo string) []string {
	return readList(filepath.Join(repo, "config", "marketplaces.txt"))
}

type MCPEntry struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	URL       string   `json:"url,omitempty"`
	Args      []string `json:"args,omitempty"`
}

func ReadMCPCatalog(repo string) ([]MCPEntry, error) {
	data, err := os.ReadFile(filepath.Join(repo, "config", "mcp.json"))
	if err != nil {
		return nil, nil
	}
	var entries []MCPEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("mcp.json malformed: %w", err)
	}
	return entries, nil
}

func HeadroomBaseURL() string {
	return "http://127.0.0.1:" + strconv.Itoa(HeadroomPort)
}

func HeadroomStatsURL() string {
	return HeadroomBaseURL() + "/stats"
}

func AuthProxyPort(repo string) int {
	v, ok := envRead(repo, "AUTH_PROXY_PORT")
	if !ok {
		return defaultAuthProxyPort
	}
	port, err := strconv.Atoi(v)
	if err != nil {
		return defaultAuthProxyPort
	}
	return port
}

func Sock(repo string) bool {
	v, _ := envRead(repo, "SOCK")
	return v == "1"
}

func WriteSock(repo string, on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return envWrite(repo, "SOCK", v)
}

func LogPath(repo string) string {
	return filepath.Join(repo, ".mirabilis", "host.log")
}

func envRead(repo, key string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(repo, ".env"))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, key+"="); ok {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

func envWrite(repo, key, value string) error {
	path := filepath.Join(repo, ".env")
	var keep []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, key+"=") {
				keep = append(keep, line)
			}
		}
	}
	out := strings.TrimRight(strings.Join(keep, "\n"), "\n")
	if out != "" {
		out += "\n"
	}
	out += key + "=" + value + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}

func readList(path string) []string {
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
