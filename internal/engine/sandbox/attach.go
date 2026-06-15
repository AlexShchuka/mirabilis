package sandbox

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const systemPromptOut = "/tmp/mirabilis-system-prompt.md"

func contextContent() string {
	var sb strings.Builder
	sb.WriteString("storage(/workspace,durable). storage(~/.claude,durable). storage(/tmp,ephemeral).\n")
	sb.WriteString("memory_root(~/.claude/memory). rules_root(~/.claude/rules).\n")
	for _, cat := range config.MemoryCategories {
		fmt.Fprintf(&sb, "memory_category(%s,%s).\n", cat.Name, cat.MemoryType)
	}
	sb.WriteString("rule: keep ~/.claude/rules/*.md only for genuine path-scoped per-file-type rules.\n")
	return sb.String()
}

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

func (s *Sandbox) SystemPromptFile(ctx context.Context) (string, error) {
	base := contextContent()
	script := fmt.Sprintf(
		`printf %%s %q >%q; nm="$HOME/.neuro-matrix/CLAUDE.md"; if [ -f "$nm" ]; then cat "$nm" >>%q; fi`,
		base, systemPromptOut, systemPromptOut,
	)
	if _, err := exec.Run(ctx, s.runner, exec.Spec{
		Argv: []string{"docker", "exec", ContainerName, "bash", "-lc", script},
		Dir:  s.repo,
	}); err != nil {
		return "", fmt.Errorf("write system prompt: %w", err)
	}
	return systemPromptOut, nil
}
