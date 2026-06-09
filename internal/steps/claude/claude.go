package claude

import (
	"context"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

type claudeStep struct{}

func (claudeStep) Check(context.Context, runner.Runner) (bool, error) { return false, nil }

func (claudeStep) Run(ctx context.Context, r runner.Runner) error {
	const script = `f="$HOME/.claude.json"
[ -f "$f" ] || printf '{}' >"$f"
tmp="$(mktemp)"
jq '.projects["/workspace"].hasTrustDialogAccepted = true
  | .hasCompletedOnboarding = true
  | .bypassPermissionsModeAccepted = true' "$f" >"$tmp" && mv "$tmp" "$f"`
	_, err := r.Container(ctx, "bash", "-lc", script)
	return err
}

type themeStep struct{}

func (themeStep) Check(ctx context.Context, r runner.Runner) (bool, error) {
	out, _ := r.Container(ctx, "bash", "-lc", `jq -r '.theme // empty' "$HOME/.claude/settings.json" 2>/dev/null`)
	return strings.TrimSpace(out) != "", nil
}

func (themeStep) Run(ctx context.Context, r runner.Runner) error {
	_, err := r.Container(ctx, "bash", "-lc", `
		th="$(cat "$HOME/.claude/.mirabilis-theme" 2>/dev/null)"; [ -n "$th" ] || th=auto
		tmp="$(mktemp)"
		jq --arg t "$th" '.theme=$t' "$HOME/.claude/settings.json" >"$tmp" && mv "$tmp" "$HOME/.claude/settings.json" || rm -f "$tmp"`)
	return err
}

func Steps() []pipeline.Registered {
	return []pipeline.Registered{
		{
			Meta: pipeline.StepMeta{
				Name:     "claude",
				Title:    "Claude config",
				Detail:   "configuring Claude inside the container",
				Deps:     []string{"prepare"},
				Retry:    pipeline.RetryNone,
				Optional: true,
				Timeout:  30 * time.Second,
			},
			Impl: claudeStep{},
		},
		{
			Meta: pipeline.StepMeta{
				Name:     "theme",
				Title:    "Theme",
				Detail:   "applying theme",
				Deps:     []string{"prepare"},
				Retry:    pipeline.RetryNone,
				Optional: true,
				Timeout:  30 * time.Second,
			},
			Impl: themeStep{},
		},
	}
}
