package main

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
)

func handoff(r Runner) error {
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
	return syscall.Exec(dk, handoffArgv(dk, strings.TrimSpace(ght), spf), composeEnv(r.Repo()))
}

// handoffArgv launches Claude with `docker exec -it`: the -i/-t flags give it a
// real PTY whose size and SIGWINCH Docker forwards, so the TUI renders
// full-height with the input pinned to the bottom like a native terminal.
// `devcontainer exec` did not propagate a sized PTY through the launcher, so
// the UI floated.
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
