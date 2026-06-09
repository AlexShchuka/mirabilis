package provision

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func readHarnessChoice() string {
	data, err := os.ReadFile(filepath.Join(claudeDir(), ".mirabilis-harness"))
	if err != nil {
		return "install"
	}
	return strings.TrimSpace(string(data))
}

func readPluginCatalog(cfg config.Config) []string {
	f, err := os.Open(cfg.PluginsTxt())
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func readDisabledPlugins() []string {
	data, err := os.ReadFile(filepath.Join(claudeDir(), ".mirabilis-plugins-disabled"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func EnsurePlugins(ctx context.Context, r runner.Runner, cfg config.Config) error {
	if _, err := r.Container(ctx, "bash", "-lc", "command -v claude"); err != nil {
		return nil
	}

	catalog := readPluginCatalog(cfg)
	if len(catalog) == 0 {
		return nil
	}

	_, _ = r.Container(ctx, "bash", "-lc", "mkdir -p \"$HOME/.cache/tmp\"")

	for _, market := range []string{"anthropics/claude-plugins-official", "jarrodwatts/claude-hud"} {
		if _, err := r.Container(ctx, "claude", "plugin", "marketplace", "add", market); err != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: marketplace add %s: %v\n", market, err)
		}
	}

	disabled := readDisabledPlugins()
	disabledSet := make(map[string]bool, len(disabled))
	for _, d := range disabled {
		disabledSet[d] = true
	}

	listed, _ := r.Container(ctx, "claude", "plugin", "list")

	for _, p := range catalog {
		if disabledSet[p] {
			continue
		}
		pluginBase := p
		if i := strings.Index(p, "@"); i >= 0 {
			pluginBase = p[:i]
		}
		if strings.Contains(listed, pluginBase) {
			continue
		}
		if _, err := r.Container(ctx, "bash", "-lc",
			fmt.Sprintf(`TMPDIR="$HOME/.cache/tmp" claude plugin install %s --scope user`, p)); err != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: plugin install %s: %v\n", p, err)
		}
	}

	return writeEnabledPlugins(ctx, r, cfg)
}

func writeEnabledPlugins(_ context.Context, _ runner.Runner, cfg config.Config) error {
	harness := readHarnessChoice()
	catalog := readPluginCatalog(cfg)
	disabled := readDisabledPlugins()
	disabledSet := make(map[string]bool, len(disabled))
	for _, d := range disabled {
		disabledSet[d] = true
	}

	enabled := make(map[string]any)
	if harness != "skip" {
		enabled["neuro-matrix@neuro-matrix"] = true
	}
	for _, p := range catalog {
		if !disabledSet[p] {
			enabled[p] = true
		}
	}

	dest := settingsPath()
	if _, err := os.Stat(dest); err != nil {
		return nil
	}
	m, err := readJSON(dest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: read settings for enabledPlugins: %v\n", err)
		return nil
	}
	m["enabledPlugins"] = enabled
	if err := writeJSON(dest, m); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: write enabledPlugins: %v\n", err)
	}
	return nil
}
