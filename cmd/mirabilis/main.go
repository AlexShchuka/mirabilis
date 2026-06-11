package main

import (
	"context"
	"fmt"
	"os"

	"github.com/AlexShchuka/mirabilis/internal/app"
	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/hooks"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
	"github.com/AlexShchuka/mirabilis/internal/telegram"
)

var version = "unknown"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "mirabilis: %v\n", err)
		os.Exit(1)
	}
}

func effectiveVersion() string {
	if version != "unknown" {
		return version
	}
	if v := os.Getenv("MIRABILIS_VERSION"); v != "" {
		return v
	}
	return "unknown"
}

func run(args []string) error {
	ctx := context.Background()
	if len(args) == 0 {
		return app.Run(ctx)
	}
	switch args[0] {
	case "-version", "--version", "version":
		fmt.Println(effectiveVersion())
		return nil
	case "-h", "-help", "--help", "help":
		fmt.Print("usage: mirabilis [command]\n\ncommands:\n  provision   run a provisioning phase inside the container\n  hook        dispatch a git hook by name\n  tg-outbox   run the host-side Telegram outbox watcher\n\nflags:\n  --version   print the build version and exit\n  --help      print this message and exit\n")
		return nil
	case "provision":
		return runProvision(ctx, runtime.NewLocalRunner(), args[1:])
	case "hook":
		if len(args) < 2 {
			return fmt.Errorf("hook: missing name argument")
		}
		return hooks.Dispatch(args[1])
	case "tg-outbox":
		return runTgOutbox(ctx, args[1:])
	default:
		return fmt.Errorf("unknown argument %q — run 'mirabilis --help' for usage", args[0])
	}
}

func runTgOutbox(ctx context.Context, _ []string) error {
	r := runtime.NewExecRunner()
	repo := r.Repo()

	tokenPath := provision.TelegramTokenPath()
	if _, err := os.Stat(tokenPath); err != nil {
		return fmt.Errorf("tg-outbox: bot token not found at %s — run the provision step first", tokenPath)
	}

	allowedChatID := runtime.KeychainGetTelegramChat()
	if allowedChatID == "" {
		return fmt.Errorf("tg-outbox: telegram-chat not configured — run provisioning")
	}

	queueDir := telegram.OutboxDir(repo)
	fmt.Fprintf(os.Stderr, "mirabilis tg-outbox: watching %s (chat %s)\n", queueDir, allowedChatID)

	cfg := telegram.WatcherConfig{
		QueueDir:      queueDir,
		TokenPath:     tokenPath,
		AllowedChatID: allowedChatID,
	}
	return telegram.RunWatcher(ctx, cfg)
}

func runProvision(ctx context.Context, r runner.Runner, args []string) error {
	phase := ""
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--phase" {
			phase = args[i+1]
		}
	}
	cfg := config.New("/opt/mirabilis/config")
	switch phase {
	case "create":
		return provision.Create(ctx, r, cfg)
	case "start":
		return provision.Start(ctx, r, cfg)
	case "plugins":
		return provision.EnsurePlugins(ctx, r, cfg)
	case "skills":
		return provision.EnsureSkills(ctx, r, cfg)
	default:
		return fmt.Errorf("provision: unknown --phase %q (want create, start, plugins, or skills)", phase)
	}
}
