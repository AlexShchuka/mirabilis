package provision

import (
	"context"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func EnsureHeadroom(ctx context.Context, r runner.Runner) error {
	if _, err := r.Container(ctx, "bash", "-lc", `test -x "$HOME/.headroom-venv/bin/headroom"`); err != nil {
		if _, err := r.Container(ctx, "bash", "-lc", `python3 -m venv "$HOME/.headroom-venv"`); err != nil {
			warn("headroom venv", err)
			return nil
		}
		if _, err := r.Container(ctx, "bash", "-lc", `timeout 600 "$HOME/.headroom-venv/bin/pip" install "headroom-ai[mcp,proxy]"`); err != nil {
			warn("headroom pip install", err)
			return nil
		}
		if _, err := r.Container(ctx, "bash", "-lc", `mkdir -p "$HOME/.local/bin" && ln -sf "$HOME/.headroom-venv/bin/headroom" "$HOME/.local/bin/headroom"`); err != nil {
			warn("headroom symlink", err)
			return nil
		}
	}

	if out, err := r.Container(ctx, "bash", "-lc", "claude mcp get headroom"); err == nil && strings.Contains(out, ".headroom-venv/bin/headroom") {
		return nil
	}

	_, _ = r.Container(ctx, "claude", "mcp", "remove", "headroom", "--scope", "user")
	if _, err := r.Container(ctx, "bash", "-lc", `claude mcp add --scope user --transport stdio headroom -- "$HOME/.headroom-venv/bin/headroom" mcp serve`); err != nil {
		warn("headroom mcp register", err)
	}
	return nil
}
