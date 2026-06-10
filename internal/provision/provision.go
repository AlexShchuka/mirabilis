package provision

import (
	"context"
	"fmt"
	"os"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func warn(step string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[provision] WARN: %s: %v\n", step, err)
}

func ensureAll(ctx context.Context, r runner.Runner, cfg config.Config) {
	warn("settings", EnsureSettings(cfg))
	warn("theme", EnsureTheme(cfg))
	warn("apt", EnsureAptPackages(ctx, r, cfg))
	warn("memory", EnsureMemory())
	warn("git identity", EnsureGitIdentity(ctx, r))
	warn("plugins", EnsurePlugins(ctx, r, cfg))
	warn("claude-hud config", EnsureHudConfig(cfg))
	ok, err := HarnessInstalled(ctx, r)
	warn("harness check", err)
	if !ok {
		warn("harness", EnsureHarness(ctx, r))
	}
	warn("mcp", EnsureMCP(ctx, r))
	warn("skills", EnsureSkills(ctx, r))
	warn("rtk", EnsureRTK(ctx, r, cfg))
	warn("rtk config", EnsureRTKConfig(cfg))
	warn("headroom", EnsureHeadroom(ctx, r))
}

func Create(ctx context.Context, r runner.Runner, cfg config.Config) error {
	ensureAll(ctx, r, cfg)
	return nil
}

func Start(ctx context.Context, r runner.Runner, cfg config.Config) error {
	ensureAll(ctx, r, cfg)
	warn("harness symlink re-assert", relinkHarness(ctx, r))
	return nil
}
