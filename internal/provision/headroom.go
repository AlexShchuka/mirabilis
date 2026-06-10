package provision

import (
	"context"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func EnsureHeadroom(ctx context.Context, r runner.Runner) error {
	if _, err := r.Container(ctx, "bash", "-lc", "headroom mcp status"); err == nil {
		return nil
	}

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

	if _, err := r.Container(ctx, "bash", "-lc", "headroom mcp install"); err != nil {
		warn("headroom mcp install", err)
	}
	return nil
}
