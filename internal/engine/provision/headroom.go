package provision

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type headroomStep struct {
	d Deps
}

func (s *headroomStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     "headroom",
		Title:    "Headroom proxy",
		Optional: true,
		Kind:     pipeline.Auto,
		Timeout:  15 * time.Minute,
	}
}

func (s *headroomStep) Check(ctx context.Context) (bool, error) {
	if !s.installed(ctx) {
		return false, nil
	}
	if s.d.ProxyAddr != "" && s.upstreamOnDisk() != s.d.ProxyAddr {
		return false, nil
	}
	if !s.reachable(ctx) {
		return false, nil
	}
	return s.mcpRegistered(ctx), nil
}

func (s *headroomStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if err := s.install(ctx, out); err != nil {
		return err
	}
	upstreamChanged := false
	if s.d.ProxyAddr != "" && s.upstreamOnDisk() != s.d.ProxyAddr {
		if err := os.WriteFile(s.d.upstreamPath(), []byte(s.d.ProxyAddr+"\n"), 0o644); err != nil {
			return fmt.Errorf("write upstream: %w", err)
		}
		upstreamChanged = true
	}
	if s.reachable(ctx) && upstreamChanged {
		s.runScript(ctx, out, `pkill -f "headroom proxy" || true`)
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if !s.reachable(ctx) {
		upstream := s.upstreamOnDisk()
		startEnv := []string{}
		if upstream != "" {
			startEnv = []string{"ANTHROPIC_TARGET_API_URL=" + upstream}
		}
		startScript := fmt.Sprintf(`setsid nohup %q proxy --mode cache >"$HOME/.headroom-proxy.log" 2>&1 &`,
			s.d.headroomBin())
		for ev := range s.d.Runner.Stream(ctx, exec.Spec{Argv: []string{"bash", "-lc", startScript}, Env: startEnv}) {
			pipeline.Forward("headroom", out, ev)
			if ev.Kind == exec.KindExited {
				break
			}
		}
		if !s.poll(ctx, out) {
			return fmt.Errorf("headroom proxy did not come up")
		}
	}
	if !s.mcpRegistered(ctx) {
		s.runScript(ctx, out, `claude mcp remove headroom --scope user >/dev/null 2>&1 || true`)
		if err := s.runScriptErr(ctx, out,
			fmt.Sprintf(`claude mcp add --scope user --transport stdio headroom -- %q mcp serve`, s.d.headroomBin())); err != nil {
			return fmt.Errorf("headroom mcp register: %w", err)
		}
	}
	return nil
}

func (s *headroomStep) installed(ctx context.Context) bool {
	return s.scriptOK(ctx, fmt.Sprintf(`test -x %q`, s.d.headroomBin()))
}

func (s *headroomStep) install(ctx context.Context, out chan<- pipeline.Event) error {
	if s.installed(ctx) {
		return nil
	}
	if err := s.runScriptErr(ctx, out, `python3 -m venv "$HOME/.headroom-venv"`); err != nil {
		return fmt.Errorf("headroom venv: %w", err)
	}
	if err := s.runScriptErr(ctx, out, `timeout 600 "$HOME/.headroom-venv/bin/pip" install "headroom-ai[mcp,proxy]"`); err != nil {
		return fmt.Errorf("headroom pip install: %w", err)
	}
	if err := s.runScriptErr(ctx, out, `mkdir -p "$HOME/.local/bin" && ln -sf "$HOME/.headroom-venv/bin/headroom" "$HOME/.local/bin/headroom"`); err != nil {
		return fmt.Errorf("headroom symlink: %w", err)
	}
	return nil
}

func (s *headroomStep) upstreamOnDisk() string {
	data, err := os.ReadFile(s.d.upstreamPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (s *headroomStep) reachable(ctx context.Context) bool {
	return s.scriptOK(ctx, `curl -fsS `+headroomStatsURL+` >/dev/null 2>&1`)
}

func (s *headroomStep) mcpRegistered(ctx context.Context) bool {
	outStr, err := exec.Run(ctx, s.d.Runner, exec.Spec{Argv: []string{"bash", "-lc", "claude mcp get headroom"}})
	return err == nil && strings.Contains(outStr, ".headroom-venv/bin/headroom")
}

func (s *headroomStep) poll(ctx context.Context, out chan<- pipeline.Event) bool {
	script := fmt.Sprintf(`for i in $(seq 1 %d); do curl -fsS %s >/dev/null 2>&1 && exit 0; sleep 1; done; exit 1`,
		headroomPollLimit, headroomStatsURL)
	return s.runScriptErr(ctx, out, script) == nil
}

func (s *headroomStep) scriptOK(ctx context.Context, script string) bool {
	_, err := exec.Run(ctx, s.d.Runner, exec.Spec{Argv: []string{"bash", "-lc", script}})
	return err == nil
}

func (s *headroomStep) runScript(ctx context.Context, out chan<- pipeline.Event, script string) {
	_ = s.runScriptErr(ctx, out, script)
}

func (s *headroomStep) runScriptErr(ctx context.Context, out chan<- pipeline.Event, script string) error {
	for ev := range s.d.Runner.Stream(ctx, exec.Spec{Argv: []string{"bash", "-lc", script}}) {
		pipeline.Forward("headroom", out, ev)
		if ev.Kind == exec.KindExited {
			return ev.Err
		}
	}
	return nil
}
