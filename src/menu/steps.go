package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func buildSteps() []Step {
	return []Step{
		{
			Name: "update", Title: "Обновление (origin/main)", Retry: retryNet, Optional: true,
			Check: checkUpToDate, Run: runPull,
		},
		{
			Name: "stacks", Title: "Стеки сборки",
			Check: checkStacks, ExecCmd: func(r Runner) *exec.Cmd { return selfCmd("stacks") },
		},
		{
			Name: "prepare", Title: "Контейнер", Deps: []string{"stacks", "update"}, Retry: retryNet,
			Check: checkContainerReady, Run: runPrepare,
		},
		{
			Name: "theme", Title: "Тема", Deps: []string{"prepare"}, Retry: retryNone, Optional: true,
			Check: checkTheme, Run: runApplyTheme,
		},
		{
			Name: "harness", Title: "neuro-matrix", Deps: []string{"prepare"}, Retry: retryNet, Optional: true,
			Check: checkHarness, Run: runHarness,
		},
		{
			Name: "gh", Title: "GitHub sign-in", Deps: []string{"prepare"}, Optional: true,
			Check: checkGitHub, ExecCmd: func(r Runner) *exec.Cmd {
				return containerCmd(r, "gh", "auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web")
			},
		},
		{
			Name: "preflight", Title: "Проверка окружения", Deps: []string{"prepare", "harness", "gh"}, Retry: retryNone,
			Check: alwaysRun, Run: runPreflight,
		},
	}
}

func alwaysRun(context.Context, Runner) (bool, error) { return false, nil }

func checkUpToDate(ctx context.Context, r Runner) (bool, error) {
	_, _ = r.Host(ctx, "git", "-C", r.Repo(), "fetch", "-q", "origin")
	out, err := r.Host(ctx, "git", "-C", r.Repo(), "rev-list", "--count", "HEAD..origin/main")
	if err != nil {
		return true, nil
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n == 0, nil
}

func runPull(ctx context.Context, r Runner) error {
	if dirty, _ := r.Host(ctx, "git", "-C", r.Repo(), "status", "--porcelain"); dirty != "" {
		return fmt.Errorf("local changes present — commit or stash before updating")
	}
	if _, err := r.Host(ctx, "git", "-C", r.Repo(), "checkout", "main"); err != nil {
		return err
	}
	_, err := r.Host(ctx, "git", "-C", r.Repo(), "pull", "--ff-only")
	return err
}

func readStacks(repo string) (string, bool) {
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

func checkStacks(_ context.Context, r Runner) (bool, error) {
	_, defined := readStacks(r.Repo())
	return defined, nil
}

func readStackCatalog(repo string) []string {
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

func writeStacks(repo, csv string) error {
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

func containerEnvValue(ctx context.Context, r Runner, key string) string {
	out, err := r.Host(ctx, "docker", "inspect", "-f", "{{range .Config.Env}}{{println .}}{{end}}", "mirabilis")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func containerRunning(ctx context.Context, r Runner) bool {
	out, _ := r.Host(ctx, "docker", "container", "inspect", "-f", "{{.State.Running}}", "mirabilis")
	return strings.TrimSpace(out) == "true"
}

func isStale(ctx context.Context, r Runner) bool {
	cont := containerEnvValue(ctx, r, "MIRABILIS_VERSION")
	if cont == "" {
		return true
	}
	wantStacks, _ := readStacks(r.Repo())
	if containerEnvValue(ctx, r, "MIRABILIS_STACKS") != wantStacks {
		return true
	}
	src, err := r.Host(ctx, "git", "-C", r.Repo(), "rev-parse", "--short", "HEAD")
	if err != nil || src == "" {
		return false
	}
	return cont != src
}

func checkContainerReady(ctx context.Context, r Runner) (bool, error) {
	return containerRunning(ctx, r) && !isStale(ctx, r), nil
}

func runPrepare(ctx context.Context, r Runner) error {
	if isStale(ctx, r) {
		_, _ = r.Host(ctx, "docker", "image", "rm", "mirabilis:local")
	}
	cmd := exec.CommandContext(ctx, "devcontainer", "up", "--workspace-folder", r.Repo())
	cmd.Env = composeEnv(r.Repo())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("devcontainer up failed: %s", lastLines(string(out), 12))
	}
	return nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func checkTheme(ctx context.Context, r Runner) (bool, error) {
	out, _ := r.Container(ctx, "bash", "-lc", `jq -r '.theme // empty' "$HOME/.claude/settings.json" 2>/dev/null`)
	return strings.TrimSpace(out) != "", nil
}

func runApplyTheme(ctx context.Context, r Runner) error {
	_, err := r.Container(ctx, "bash", "-lc", `
		th="$(cat "$HOME/.claude/.mirabilis-theme" 2>/dev/null)"; [ -n "$th" ] || th=auto
		tmp="$(mktemp)"
		jq --arg t "$th" '.theme=$t' "$HOME/.claude/settings.json" >"$tmp" && mv "$tmp" "$HOME/.claude/settings.json" || rm -f "$tmp"`)
	return err
}

func checkHarness(ctx context.Context, r Runner) (bool, error) {
	pref, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`)
	if strings.TrimSpace(pref) == "skip" {
		return true, nil
	}
	_, err := r.Container(ctx, "bash", "-lc", `claude plugin list 2>/dev/null | grep -q neuro-matrix`)
	return err == nil, nil
}

func runHarness(ctx context.Context, r Runner) error {
	_, err := r.Container(ctx, "bash", "/usr/local/bin/harness-reinstall.sh")
	return err
}

func checkGitHub(ctx context.Context, r Runner) (bool, error) {
	_, err := r.Container(ctx, "gh", "auth", "status")
	return err == nil, nil
}

func runPreflight(ctx context.Context, r Runner) error {
	if ip, _ := r.Container(ctx, "curl", "-s", "-m", "8", "https://api.ipify.org"); strings.TrimSpace(ip) == "" {
		return fmt.Errorf("egress: the container has no outbound network")
	}
	code, _ := r.Container(ctx, "curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "-m", "12", "https://api.anthropic.com/v1/models")
	switch strings.TrimSpace(code) {
	case "200", "401", "403":
		return nil
	case "", "000":
		return fmt.Errorf("api.anthropic.com: unreachable")
	default:
		return fmt.Errorf("api.anthropic.com: HTTP %s", strings.TrimSpace(code))
	}
}
