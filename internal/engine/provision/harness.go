package provision

import (
	"context"
	"errors"
	"fmt"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	harnessProbeScript = `claude plugin list 2>/dev/null | grep -q neuro-matrix`
	relinkScript       = `NM_DIR="$(printf '%s\n' "$HOME"/.claude/plugins/cache/*/neuro-matrix/*/ | sort -V | tail -n1)"; [ -d "$NM_DIR" ] && ln -sfn "${NM_DIR%/}" "$HOME/.neuro-matrix"; L='export CLAUDE_PLUGIN_ROOT="$HOME/.neuro-matrix"'; grep -qxF "$L" "$HOME/.bashrc" 2>/dev/null || printf '%s\n' "$L" >>"$HOME/.bashrc"`
)

type harnessStep struct {
	d Deps
}

func (s *harnessStep) Meta() pipeline.Meta { return installMeta("harness", "neuro-matrix harness") }

func (s *harnessStep) installed(ctx context.Context) bool {
	if s.d.harnessChoice() == harnessSkip {
		return true
	}
	return s.d.scriptOK(ctx, harnessProbeScript)
}

func (s *harnessStep) Check(ctx context.Context) (bool, error) {
	return s.installed(ctx), nil
}

func (s *harnessStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	var errs []error
	if !s.installed(ctx) {
		errs = append(errs, s.install(ctx, out))
	}
	if err := s.d.streamScript(ctx, "harness", out, relinkScript); err != nil {
		errs = append(errs, fmt.Errorf("neuro-matrix symlink: %w", err))
	}
	return errors.Join(errs...)
}

func (s *harnessStep) install(ctx context.Context, out chan<- pipeline.Event) error {
	if !s.d.scriptOK(ctx, "command -v claude") {
		return nil
	}
	var errs []error
	if err := s.d.stream(ctx, "harness", out, "claude", "plugin", "marketplace", "add", "AlexShchuka/neuro-matrix"); err != nil {
		if err2 := s.d.stream(ctx, "harness", out, "claude", "plugin", "marketplace", "update", "neuro-matrix"); err2 != nil {
			errs = append(errs, fmt.Errorf("marketplace add/update neuro-matrix: %w", err2))
		}
	}
	if err := s.d.stream(ctx, "harness", out, "claude", "plugin", "install", "neuro-matrix@neuro-matrix", "--scope", "user"); err != nil {
		errs = append(errs, fmt.Errorf("plugin install neuro-matrix: %w", err))
	}
	if err := s.d.stream(ctx, "harness", out, "claude", "plugin", "update", "neuro-matrix@neuro-matrix"); err != nil {
		errs = append(errs, fmt.Errorf("plugin update neuro-matrix: %w", err))
	}
	if !s.d.scriptOK(ctx, harnessProbeScript) {
		errs = append(errs, errors.New("neuro-matrix not installed after reinstall"))
	}
	return errors.Join(errs...)
}
