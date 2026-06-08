package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"
)

func RunPipeline() error {
	ctx := context.Background()
	r := newExecRunner()
	if err := ensureDocker(ctx); err != nil {
		return err
	}
	p := newPipeline(ctx, r, buildSteps())
	final, err := tea.NewProgram(p, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return err
	}
	if fp, ok := final.(*pipeline); ok && fp.failed {
		return fmt.Errorf("a launch step failed — see above")
	}
	return handoff(r)
}

func handoff(r Runner) error {
	ctx := context.Background()
	ght, _ := r.Container(ctx, "gh", "auth", "token")
	spf, _ := r.Container(ctx, "bash", "-lc", systemPromptScript)
	if spf = strings.TrimSpace(spf); spf == "" {
		spf = "/opt/mirabilis/config/sandbox-context.md"
	}
	dc, err := exec.LookPath("devcontainer")
	if err != nil {
		return err
	}
	argv := []string{
		dc, "exec", "--workspace-folder", r.Repo(),
		"env", "GITHUB_PERSONAL_ACCESS_TOKEN=" + strings.TrimSpace(ght),
		"COLORTERM=truecolor", "TERM=xterm-256color",
		"claude", "--dangerously-skip-permissions", "--append-system-prompt-file", spf,
	}
	return syscall.Exec(dc, argv, composeEnv(r.Repo()))
}

const systemPromptScript = `sbx=/opt/mirabilis/config/sandbox-context.md; out=/tmp/mirabilis-system-prompt.md
nm="$HOME/.neuro-matrix/CLAUDE.md"
if [ -f "$nm" ]; then cat "$sbx" "$nm" >"$out" && { printf %s "$out"; exit 0; }; fi
printf %s "$sbx"`
