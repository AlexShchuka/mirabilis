package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/provision"
)

func SessionStart() error {
	_, _ = io.ReadAll(os.Stdin)

	ctx := context.Background()
	if err := runScript(ctx, "gh auth setup-git"); err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: gh auth setup-git: %v\n", err)
	}
	if err := ensureGHCredentialAuthority(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: credential-authority: %v\n", err)
	}

	proxyCtx, proxyCancel := context.WithTimeout(ctx, 75*time.Second)
	defer proxyCancel()
	ensureProxyForSession(proxyCtx)

	memDir := filepath.Join(home(), ".claude", "memory")
	idx, _ := memoryIndex(memDir)
	tgCtx := sessionTelegramContext(repoRoot())

	additionalContext := joinContext(idx, ecosystemContext(), tgCtx)

	if idx != "" {
		if err := os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(idx), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "[hook] WARN: write MEMORY.md: %v\n", err)
		}
	}

	payload, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "SessionStart",
			"additionalContext": additionalContext,
		},
	})
	_, _ = os.Stdout.Write(payload)
	return nil
}

func ensureProxyForSession(ctx context.Context) {
	data, err := os.ReadFile(filepath.Join(home(), ".claude", provision.UpstreamFileName))
	if err != nil {
		return
	}
	if proxyAlive(ctx) {
		return
	}
	if !startHeadroom(ctx, strings.TrimSpace(string(data))) {
		fmt.Fprintln(os.Stderr, "[hook] WARN: headroom proxy not ready")
	}
}

func proxyAlive(ctx context.Context) bool {
	return runScript(ctx, `curl -fsS `+headroomStatsURL+` >/dev/null 2>&1`) == nil
}

func startHeadroom(ctx context.Context, upstream string) bool {
	startEnv := []string{}
	if upstream != "" {
		startEnv = []string{"ANTHROPIC_TARGET_API_URL=" + upstream}
	}
	start := fmt.Sprintf(`setsid nohup %q proxy --mode %q >"$HOME/.headroom-proxy.log" 2>&1 &`,
		filepath.Join(home(), headroomVenvRel), config.HeadroomMode(repoRoot()))
	startSpec := exec.Spec{Argv: []string{"bash", "-lc", start}, Env: startEnv}
	if _, err := exec.Run(ctx, runner, startSpec); err != nil {
		return false
	}
	poll := fmt.Sprintf(`for i in $(seq 1 %d); do curl -fsS %s >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1`,
		headroomPollLimit, headroomStatsURL)
	return runScript(ctx, poll) == nil
}

func runScript(ctx context.Context, script string) error {
	_, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"bash", "-lc", script}})
	return err
}

func ghOnlyChain(got string) bool {
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	return len(lines) >= 2 && lines[0] == "" && strings.Contains(lines[len(lines)-1], "gh auth git-credential")
}

func ensureGHCredentialAuthority(ctx context.Context) error {
	got, _ := exec.Run(ctx, runner, exec.Spec{
		Argv: []string{"git", "config", "--get-all", "credential.https://github.com.helper"},
	})
	if ghOnlyChain(got) {
		return nil
	}
	ghHelper, err := exec.Run(ctx, runner, exec.Spec{
		Argv: []string{"bash", "-lc", `gh_path=$(command -v gh) && [ -n "$gh_path" ] && echo "!$gh_path auth git-credential"`},
	})
	if err != nil {
		return fmt.Errorf("locate gh binary: %w", err)
	}
	helper := strings.TrimSpace(ghHelper)
	for _, host := range []string{"https://github.com", "https://gist.github.com"} {
		key := "credential." + host + ".helper"
		if _, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "config", "--global", "--replace-all", key, ""}}); err != nil {
			return fmt.Errorf("reset credential helper for %s: %w", host, err)
		}
		if _, err := exec.Run(ctx, runner, exec.Spec{Argv: []string{"git", "config", "--global", "--add", key, helper}}); err != nil {
			return fmt.Errorf("set credential helper for %s: %w", host, err)
		}
	}
	got2, _ := exec.Run(ctx, runner, exec.Spec{
		Argv: []string{"git", "config", "--get-all", "credential.https://github.com.helper"},
	})
	if !ghOnlyChain(got2) {
		return fmt.Errorf("github.com credential.helper still does not resolve to gh-only after reassertion; got %q", strings.TrimSpace(got2))
	}
	return nil
}
