package sandbox

import (
	"context"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const defaultSystemPromptFile = "/opt/mirabilis/config/sandbox-context.md"

const systemPromptScript = `sbx=/opt/mirabilis/config/sandbox-context.md; out=/tmp/mirabilis-system-prompt.md
nm="$HOME/.neuro-matrix/CLAUDE.md"
if [ -f "$nm" ]; then cat "$sbx" "$nm" >"$out" && { printf %s "$out"; exit 0; }; fi
printf %s "$sbx"`

func BuildAttachArgv(systemPromptFile string) []string {
	return []string{
		"docker", "exec", "-it",
		"-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
		"-e", "COLORTERM=truecolor",
		"-e", "TERM=xterm-256color",
		ContainerName,
		"claude", "--dangerously-skip-permissions", "--append-system-prompt-file", systemPromptFile,
	}
}

func (s *Sandbox) SystemPromptFile(ctx context.Context) string {
	out, err := exec.Run(ctx, s.runner, exec.Spec{
		Argv: []string{"docker", "exec", ContainerName, "bash", "-lc", systemPromptScript},
		Dir:  s.repo,
	})
	out = strings.TrimSpace(out)
	if err != nil || out == "" {
		return defaultSystemPromptFile
	}
	return out
}
