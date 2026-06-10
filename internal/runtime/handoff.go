package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

const (
	exitAltScreen     = "\x1b[?1049l"
	resetScrollRegion = "\x1b[r"
	showCursor        = "\x1b[?25h"
	clearScreenHome   = "\x1b[2J\x1b[H"
)

func resetTerminal(w io.Writer) {
	_, _ = io.WriteString(w, exitAltScreen+resetScrollRegion+clearScreenHome+showCursor)
}

func resolveGHToken(r runner.Runner) (string, error) {
	ctx := context.Background()
	token, err := r.Container(ctx, "gh", "auth", "token")
	token = strings.TrimSpace(token)
	if err != nil || token == "" {
		return "", fmt.Errorf("GitHub token is not available — sign in with gh auth login first")
	}
	return token, nil
}

func Handoff(r runner.Runner) error {
	ght, err := resolveGHToken(r)
	if err != nil {
		return err
	}
	ctx := context.Background()
	spf, _ := r.Container(ctx, "bash", "-lc", systemPromptScript)
	if spf = strings.TrimSpace(spf); spf == "" {
		spf = "/opt/mirabilis/config/sandbox-context.md"
	}
	dk, err := exec.LookPath("docker")
	if err != nil {
		return err
	}
	resetTerminal(os.Stderr)
	return syscall.Exec(dk, handoffArgv(dk, spf), handoffEnv(ComposeEnv(r.Repo()), ght))
}

func handoffArgv(docker, spf string) []string {
	return []string{
		docker, "exec", "-it",
		"-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
		"-e", "COLORTERM=truecolor",
		"-e", "TERM=xterm-256color",
		"mirabilis",
		"claude", "--dangerously-skip-permissions", "--append-system-prompt-file", spf,
	}
}

func handoffEnv(base []string, token string) []string {
	out := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if strings.HasPrefix(kv, "GITHUB_PERSONAL_ACCESS_TOKEN=") || kv == "GITHUB_PERSONAL_ACCESS_TOKEN" {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "GITHUB_PERSONAL_ACCESS_TOKEN="+token)
}

const systemPromptScript = `sbx=/opt/mirabilis/config/sandbox-context.md; out=/tmp/mirabilis-system-prompt.md
nm="$HOME/.neuro-matrix/CLAUDE.md"
if [ -f "$nm" ]; then cat "$sbx" "$nm" >"$out" && { printf %s "$out"; exit 0; }; fi
printf %s "$sbx"`
