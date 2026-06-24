package provision

import (
	"context"
	"fmt"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	mathToolSympy = "sympy"
	mathToolZ3    = "z3"
	mathToolLean  = "lean"
	mathToolCoq   = "coq"

	mathVenvProbe = `test -x "$HOME/.mathtools-venv/bin/python"`
	mathVenvMake  = `python3 -m venv "$HOME/.mathtools-venv"`
	mathPipInst   = `timeout 600 "$HOME/.mathtools-venv/bin/pip" install sympy z3-solver`
	mathPipLink   = `mkdir -p "$HOME/.local/bin" && ln -sf "$HOME/.mathtools-venv/bin/z3" "$HOME/.local/bin/z3"`
	mathLeanProbe = `command -v elan`
	mathLeanInst  = `curl -sSf https://raw.githubusercontent.com/leanprover/elan/master/elan-init.sh | sh -s -- -y`
	mathCoqProbe  = `command -v coqc`
	mathCoqInst   = `sudo apt-get install -y coq`
)

type mathToolsStep struct {
	d Deps
}

func (s *mathToolsStep) Meta() pipeline.Meta {
	return installMeta("mathtools", "Math anchor toolchain")
}

func (s *mathToolsStep) requested() map[string]bool {
	lo, ok := s.d.loadout()
	if !ok {
		return nil
	}
	want := map[string]bool{}
	for _, t := range lo.Tools {
		switch t {
		case mathToolSympy, mathToolZ3, mathToolLean, mathToolCoq:
			want[t] = true
		}
	}
	return want
}

func (s *mathToolsStep) Check(ctx context.Context) (bool, error) {
	want := s.requested()
	if len(want) == 0 {
		return true, nil
	}
	if (want[mathToolSympy] || want[mathToolZ3]) && !s.venvInstalled(ctx) {
		return false, nil
	}
	if want[mathToolLean] && !s.leanInstalled(ctx) {
		return false, nil
	}
	return true, nil
}

func (s *mathToolsStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	want := s.requested()
	if len(want) == 0 {
		return nil
	}
	if (want[mathToolSympy] || want[mathToolZ3]) && !s.venvInstalled(ctx) {
		if err := s.d.streamScript(ctx, "mathtools", out, mathVenvMake); err != nil {
			return fmt.Errorf("mathtools venv: %w", err)
		}
		if err := s.d.streamScript(ctx, "mathtools", out, mathPipInst); err != nil {
			return fmt.Errorf("mathtools pip install: %w", err)
		}
		if err := s.d.streamScript(ctx, "mathtools", out, mathPipLink); err != nil {
			return fmt.Errorf("mathtools z3 symlink: %w", err)
		}
	}
	if want[mathToolLean] && !s.leanInstalled(ctx) {
		if err := s.d.streamScript(ctx, "mathtools", out, mathLeanInst); err != nil {
			return fmt.Errorf("mathtools lean install: %w", err)
		}
	}
	if want[mathToolCoq] && !s.coqInstalled(ctx) {
		if err := s.d.streamScript(ctx, "mathtools", out, mathCoqInst); err != nil {
			s.d.Log.Warn("mathtools: coq install failed, continuing", "err", err)
		}
	}
	return nil
}

func (s *mathToolsStep) venvInstalled(ctx context.Context) bool {
	return s.d.scriptOK(ctx, mathVenvProbe)
}

func (s *mathToolsStep) leanInstalled(ctx context.Context) bool {
	return s.d.scriptOK(ctx, mathLeanProbe)
}

func (s *mathToolsStep) coqInstalled(ctx context.Context) bool {
	return s.d.scriptOK(ctx, mathCoqProbe)
}
