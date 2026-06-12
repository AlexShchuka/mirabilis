package steps

import (
	"context"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	harnessPrefScript   = `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`
	harnessProbeScript  = "claude plugin list 2>/dev/null | grep -q neuro-matrix"
	harnessRelinkScript = `NM_DIR="$(printf '%s\n' "$HOME"/.claude/plugins/cache/*/neuro-matrix/*/ | sort -V | tail -n1)"; [ -d "$NM_DIR" ] && ln -sfn "${NM_DIR%/}" "$HOME/.neuro-matrix"; L='export CLAUDE_PLUGIN_ROOT="$HOME/.neuro-matrix"'; grep -qxF "$L" "$HOME/.bashrc" 2>/dev/null || printf '%s\n' "$L" >>"$HOME/.bashrc"`
	harnessSkip         = "skip"
)

type harnessStep struct {
	d Deps
}

func (s *harnessStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     "harness",
		Title:    "Harness",
		Deps:     []string{"provision-start"},
		Kind:     pipeline.Auto,
		Optional: true,
		Timeout:  5 * time.Minute,
	}
}

func (s *harnessStep) Check(ctx context.Context) (bool, error) {
	pref, _ := exec.Run(ctx, s.d.Runner, exec.Spec{Argv: containerArgv("bash", "-lc", harnessPrefScript)})
	if strings.TrimSpace(pref) == harnessSkip {
		return true, nil
	}
	_, err := exec.Run(ctx, s.d.Runner, exec.Spec{Argv: containerArgv("bash", "-lc", harnessProbeScript)})
	return err == nil, nil
}

func (s *harnessStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if _, err := exec.Run(ctx, s.d.Runner, exec.Spec{Argv: containerArgv("bash", "-lc", "command -v claude")}); err != nil {
		return nil
	}
	if err := s.stream(ctx, out, "claude", "plugin", "marketplace", "add", "AlexShchuka/neuro-matrix"); err != nil {
		_ = s.stream(ctx, out, "claude", "plugin", "marketplace", "update", "neuro-matrix")
	}
	_ = s.stream(ctx, out, "claude", "plugin", "install", "neuro-matrix@neuro-matrix", "--scope", "user")
	_ = s.stream(ctx, out, "claude", "plugin", "update", "neuro-matrix@neuro-matrix")
	_ = s.stream(ctx, out, "bash", "-lc", harnessProbeScript)
	_ = s.stream(ctx, out, "bash", "-lc", harnessRelinkScript)
	return nil
}

func (s *harnessStep) stream(ctx context.Context, out chan<- pipeline.Event, args ...string) error {
	return stream("harness", out, s.d.Runner.Stream(ctx, exec.Spec{Argv: containerArgv(args...)}))
}
