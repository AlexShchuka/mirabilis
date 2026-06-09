package runtime

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	gort "runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

type execRunner struct{ repo string }

func repoRoot() string {
	if r := os.Getenv("MIRABILIS_REPO"); r != "" {
		return r
	}
	exe, err := os.Executable()
	if err == nil {
		if resolved, e := filepath.EvalSymlinks(exe); e == nil {
			exe = resolved
		}
		return filepath.Clean(filepath.Join(filepath.Dir(exe), ".."))
	}
	wd, _ := os.Getwd()
	return wd
}

func NewExecRunner() runner.Runner { return &execRunner{repo: repoRoot()} }

func (e *execRunner) Repo() string { return e.repo }

func (e *execRunner) Host(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func (e *execRunner) Container(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "devcontainer", append([]string{"exec", "--workspace-folder", e.repo}, args...)...)
	cmd.Env = ComposeEnv(e.repo)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

type localRunner struct{}

func NewLocalRunner() runner.Runner { return &localRunner{} }

func (l *localRunner) Repo() string { return "" }

func (l *localRunner) Host(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func (l *localRunner) Container(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("localRunner.Container: no args")
	}
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	return strings.TrimSpace(string(out)), err
}

func ComposeEnv(repo string) []string {
	managed := map[string]string{
		"MIRABILIS_VERSION":  GitShort(repo),
		"TELEGRAM_BOT_TOKEN": keychainGet("telegram-token"),
		"TELEGRAM_CHAT_ID":   keychainGet("telegram-chat"),
	}
	if stacks, ok := config.ReadStacks(repo); ok {
		managed["STACKS"] = stacks
	}
	var env []string
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok {
			if _, owned := managed[k]; owned {
				continue
			}
		}
		env = append(env, kv)
	}
	for k, v := range managed {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

var (
	gitShortOnce sync.Once
	gitShortVal  string
)

func GitShort(repo string) string {
	gitShortOnce.Do(func() {
		out, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
		if err != nil {
			gitShortVal = "unknown"
			return
		}
		gitShortVal = strings.TrimSpace(string(out))
	})
	return gitShortVal
}

func keychainEnv(name string) string {
	switch name {
	case "telegram-token":
		return "TELEGRAM_BOT_TOKEN"
	case "telegram-chat":
		return "TELEGRAM_CHAT_ID"
	}
	return ""
}

func keychainGet(name string) string {
	if gort.GOOS != "darwin" {
		return os.Getenv(keychainEnv(name))
	}
	account := os.Getenv("MIRABILIS_KEYCHAIN_ACCOUNT")
	if account == "" {
		if u := os.Getenv("USER"); u != "" {
			account = u
		} else {
			account = "mirabilis"
		}
	}
	out, err := exec.Command("security", "find-generic-password", "-a", account, "-s", "mirabilis-"+name+"-token", "-w").Output()
	if err != nil {
		return os.Getenv(keychainEnv(name))
	}
	return strings.TrimSpace(string(out))
}

func ContainerCmd(r runner.Runner, args ...string) *exec.Cmd {
	cmd := exec.Command("devcontainer", append([]string{"exec", "--workspace-folder", r.Repo()}, args...)...)
	cmd.Env = ComposeEnv(r.Repo())
	return cmd
}

func EnsureDocker(_ context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("Docker is not installed — run 'make bootstrap'")
	}
	if _, err := exec.LookPath("devcontainer"); err != nil {
		return fmt.Errorf("devcontainer CLI is missing — run 'make bootstrap'")
	}
	if dockerReachable() {
		return nil
	}
	if gort.GOOS != "darwin" {
		return fmt.Errorf("Docker daemon is not running")
	}
	_ = exec.Command("open", "-a", "Docker").Run()
	for i := 0; i < 60; i++ {
		if dockerReachable() {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("Docker did not come up — open Docker Desktop and run mirabilis again")
}

func dockerReachable() bool { return exec.Command("docker", "info").Run() == nil }

func ContainerRunning(ctx context.Context, r runner.Runner) bool {
	out, _ := r.Host(ctx, "docker", "container", "inspect", "-f", "{{.State.Running}}", "mirabilis")
	return strings.TrimSpace(out) == "true"
}

func ContainerExists(ctx context.Context, r runner.Runner) bool {
	_, err := r.Host(ctx, "docker", "container", "inspect", "mirabilis")
	return err == nil
}

func ContainerEnvValue(ctx context.Context, r runner.Runner, key string) string {
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

func IsStale(ctx context.Context, r runner.Runner) bool {
	cont := ContainerEnvValue(ctx, r, "MIRABILIS_VERSION")
	if cont == "" {
		return true
	}
	wantStacks, _ := config.ReadStacks(r.Repo())
	if ContainerEnvValue(ctx, r, "MIRABILIS_STACKS") != wantStacks {
		return true
	}
	src, err := r.Host(ctx, "git", "-C", r.Repo(), "rev-parse", "--short", "HEAD")
	if err != nil || src == "" {
		return false
	}
	return cont != src
}

func LastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func ResetAll(ctx context.Context, r runner.Runner) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", "mirabilis",
		"-f", filepath.Join(r.Repo(), "docker-compose.yml"),
		"down", "--rmi", "local", "-v")
	cmd.Env = ComposeEnv(r.Repo())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose down failed: %s", LastLines(string(out), 12))
	}
	return nil
}

func DoVSCode(ctx context.Context, r runner.Runner) error {
	code, err := resolveCode()
	if err != nil {
		return err
	}
	if !ContainerRunning(ctx, r) {
		up := exec.CommandContext(ctx, "devcontainer", "up", "--workspace-folder", r.Repo())
		up.Env = ComposeEnv(r.Repo())
		if e := up.Run(); e != nil {
			return e
		}
	}
	enc := hex.EncodeToString([]byte(`{"containerName":"/mirabilis"}`))
	uri := "vscode-remote://attached-container+" + enc + "/workspace"
	return exec.Command(code, "--folder-uri", uri).Run()
}

func resolveCode() (string, error) {
	if p, err := exec.LookPath("code"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, b := range []string{
		"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
		filepath.Join(home, "Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"),
		"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code",
	} {
		if fi, err := os.Stat(b); err == nil && !fi.IsDir() {
			return b, nil
		}
	}
	return "", fmt.Errorf("VS Code not found — install it from https://code.visualstudio.com")
}

func Handoff(r runner.Runner) error {
	ctx := context.Background()
	ght, _ := r.Container(ctx, "gh", "auth", "token")
	spf, _ := r.Container(ctx, "bash", "-lc", systemPromptScript)
	if spf = strings.TrimSpace(spf); spf == "" {
		spf = "/opt/mirabilis/config/sandbox-context.md"
	}
	dk, err := exec.LookPath("docker")
	if err != nil {
		return err
	}
	return syscall.Exec(dk, handoffArgv(dk, strings.TrimSpace(ght), spf), ComposeEnv(r.Repo()))
}

func handoffArgv(docker, token, spf string) []string {
	return []string{
		docker, "exec", "-it", "mirabilis",
		"env", "GITHUB_PERSONAL_ACCESS_TOKEN=" + token,
		"COLORTERM=truecolor", "TERM=xterm-256color",
		"claude", "--dangerously-skip-permissions", "--append-system-prompt-file", spf,
	}
}

const systemPromptScript = `sbx=/opt/mirabilis/config/sandbox-context.md; out=/tmp/mirabilis-system-prompt.md
nm="$HOME/.neuro-matrix/CLAUDE.md"
if [ -f "$nm" ]; then cat "$sbx" "$nm" >"$out" && { printf %s "$out"; exit 0; }; fi
printf %s "$sbx"`
