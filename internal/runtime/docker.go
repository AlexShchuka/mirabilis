package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func ContainerCmd(ctx context.Context, r runner.Runner, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "devcontainer", append([]string{"exec", "--workspace-folder", r.Repo()}, args...)...)
	cmd.Env = ComposeEnv(r.Repo())
	return cmd
}

func EnsureDocker(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is not installed — run 'make bootstrap'")
	}
	if _, err := exec.LookPath("devcontainer"); err != nil {
		return fmt.Errorf("devcontainer CLI is missing — run 'make bootstrap'")
	}
	if dockerReachable() {
		return nil
	}
	return tryStartDocker(ctx)
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

const memorySaveSubdir = ".mirabilis/saved-memory"

func MemorySavePath(repoRoot string) string {
	return filepath.Join(repoRoot, memorySaveSubdir)
}

func SaveMemory(repoRoot string) error {
	dst := MemorySavePath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("save memory: mkdir: %w", err)
	}
	_ = os.RemoveAll(dst)
	cmd := exec.Command("docker", "cp", "mirabilis:/home/node/.claude/memory", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("save memory: docker cp: %s", LastLines(string(out), 6))
	}
	return nil
}

func ResetAll(ctx context.Context, r runner.Runner, preserve bool) error {
	if preserve {
		if err := SaveMemory(r.Repo()); err != nil {
			fmt.Fprintf(os.Stderr, "[runtime] WARN: save memory before reset: %v\n", err)
		}
	}
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", "mirabilis",
		"-f", filepath.Join(r.Repo(), "docker-compose.yml"),
		"down", "--rmi", "local", "-v")
	cmd.Env = ComposeEnv(r.Repo())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose down failed: %s", LastLines(string(out), 12))
	}
	return nil
}
