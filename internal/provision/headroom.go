package provision

import (
	"context"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func EnsureHeadroom(ctx context.Context, r runner.Runner) error {
	if _, err := r.Container(ctx, "bash", "-lc", "command -v headroom"); err == nil {
		return nil
	}

	_, err := r.Container(ctx, "bash", "-lc",
		`python3 -m venv "$HOME/.headroom-venv" && "$HOME/.headroom-venv/bin/pip" install -q "headroom-ai[all]" && mkdir -p "$HOME/.local/bin" && ln -sf "$HOME/.headroom-venv/bin/headroom" "$HOME/.local/bin/headroom"`)
	if err != nil {
		warn("headroom install", err)
		return nil
	}

	if _, err := r.Container(ctx, "bash", "-lc", "headroom mcp install"); err != nil {
		warn("headroom mcp install", err)
	}
	return nil
}
