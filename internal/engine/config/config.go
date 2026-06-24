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

const DefaultLoadout = "raid"

type Loadout struct {
	Name    string
	Effort  string
	Harness bool
	Batch   bool
	Plugins []string
	MCP     []string
	Tools   []string
}

func ReadLoadout(repo string) (string, bool) { return envRead(repo, "LOADOUT") }

// LaunchBatched reports whether the active loadout opts the launch pipeline into
// the concurrent batch fast-path. Default loadouts leave it off, so the sequential
// launch path is unchanged unless a loadout sets "batch on".
func LaunchBatched(repo string) bool {
	name, ok := ReadLoadout(repo)
	if !ok || name == "" {
		name = DefaultLoadout
	}
	lo, ok := ReadLoadoutManifest(repo, name)
	return ok && lo.Batch
}

func WriteLoadout(repo, name string) error { return envWrite(repo, "LOADOUT", name) }

func ReadLoadoutCatalog(repo string) []string {
	ents, err := os.ReadDir(filepath.Join(repo, "config", "loadouts"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if name, ok := strings.CutSuffix(e.Name(), ".txt"); ok {
			out = append(out, name)
		}
	}
	return out
}

func ReadLoadoutManifest(repo, name string) (Loadout, bool) {
	path := filepath.Join(repo, "config", "loadouts", name+".txt")
	if _, err := os.Stat(path); err != nil {
		return Loadout{}, false
	}
	lo := Loadout{Name: name}
	for _, line := range readList(path) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "effort":
			if len(fields) > 1 {
				lo.Effort = fields[1]
			}
		case "harness":
			lo.Harness = len(fields) > 1 && fields[1] == "on"
		case "batch":
			lo.Batch = len(fields) > 1 && fields[1] == "on"
		case "plugins":
			lo.Plugins = fields[1:]
		case "mcp":
			lo.MCP = fields[1:]
		case "tools":
			lo.Tools = fields[1:]
		}
	}
	return lo, true
}

type SkillGroup struct {
	Name   string
	Repo   string
	Skills []string
}

func SkillGroupsFrom(path string) []SkillGroup {
	var out []SkillGroup
	for _, line := range readList(path) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		g := SkillGroup{Name: fields[0]}
		if len(fields) > 1 {
			g.Repo = fields[1]
		}
		if len(fields) > 2 {
			g.Skills = fields[2:]
		}
		out = append(out, g)
	}
	return out
}

func ReadSkillGroups(repo string) []SkillGroup {
	return SkillGroupsFrom(filepath.Join(repo, "config", "skills.txt"))
}

func ReadSkillCatalog(repo string) []string {
	var names []string
	for _, g := range ReadSkillGroups(repo) {
		names = append(names, g.Name)
	}
	return names
}

func ReadMarketplaces(repo string) []string {
	return readList(filepath.Join(repo, "config", "marketplaces.txt"))
}

type MCPEntry struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	URL       string   `json:"url,omitempty"`
	Args      []string `json:"args,omitempty"`
	Shrink    bool     `json:"shrink,omitempty"`
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

func LocalLLMEffectiveBaseURL() string {
	if v := os.Getenv("LOCAL_LLM_BASE_URL"); v != "" {
		return v
	}
	return LocalLLMBaseURL
}

func LocalLLMEffectiveModel() string {
	if v := os.Getenv("LOCAL_LLM_MODEL"); v != "" {
		return v
	}
	return LocalLLMModel
}

func LocalLLMEffectiveTimeout() time.Duration {
	if v := os.Getenv("LOCAL_LLM_TIMEOUT_S"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return LocalLLMTimeout
}

func LocalLLMEffectiveMaxTokens() int {
	if v := os.Getenv("LOCAL_LLM_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return LocalLLMMaxTokens
}

const defaultHeadroomMode = "cache"

func HeadroomMode(repo string) string {
	if v, ok := envRead(repo, "HEADROOM_MODE"); ok && v != "" {
		return allowedMode(v)
	}
	if v := os.Getenv("HEADROOM_MODE"); v != "" {
		return allowedMode(v)
	}
	return defaultHeadroomMode
}

func allowedMode(v string) string {
	switch v {
	case "cache", "token":
		return v
	default:
		return defaultHeadroomMode
	}
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
