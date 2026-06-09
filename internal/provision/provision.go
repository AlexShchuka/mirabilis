package provision

import (
	"context"
	"fmt"
	"os"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func ensureAll(ctx context.Context, r runner.Runner, cfg config.Config) {
	if err := EnsureSettings(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: settings: %v\n", err)
	}
	if err := EnsureTheme(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: theme: %v\n", err)
	}
	if err := EnsureAptPackages(ctx, r, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: apt: %v\n", err)
	}
	if err := EnsureMemoryRules(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: memory rules: %v\n", err)
	}
	if err := EnsureGitIdentity(ctx, r); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: git identity: %v\n", err)
	}
	if err := EnsurePlugins(ctx, r, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: plugins: %v\n", err)
	}
	ok, err := HarnessInstalled(ctx, r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: harness check: %v\n", err)
	}
	if !ok {
		if err := EnsureHarness(ctx, r); err != nil {
			fmt.Fprintf(os.Stderr, "[provision] WARN: harness: %v\n", err)
		}
	}
	if err := EnsureMCP(ctx, r); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: mcp: %v\n", err)
	}
	if err := EnsureSkills(ctx, r); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: skills: %v\n", err)
	}
	if err := EnsureRTK(ctx, r, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: rtk: %v\n", err)
	}
}

func Create(ctx context.Context, r runner.Runner, cfg config.Config) error {
	ensureAll(ctx, r, cfg)
	return nil
}

func Start(ctx context.Context, r runner.Runner, cfg config.Config) error {
	ensureAll(ctx, r, cfg)

	if err := relinkHarness(ctx, r); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: harness symlink re-assert: %v\n", err)
	}
	return nil
}
