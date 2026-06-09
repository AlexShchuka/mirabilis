package main

import (
	"context"
	"fmt"
	"os"

	"github.com/AlexShchuka/mirabilis/internal/app"
	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/hooks"
	"github.com/AlexShchuka/mirabilis/internal/provision"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "mirabilis: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	ctx := context.Background()
	if len(args) == 0 {
		return app.Run(ctx)
	}
	switch args[0] {
	case "provision":
		return runProvision(ctx, args[1:])
	case "hook":
		if len(args) < 2 {
			return fmt.Errorf("hook: missing name argument")
		}
		return hooks.Dispatch(args[1])
	default:
		return app.Run(ctx)
	}
}

func runProvision(ctx context.Context, args []string) error {
	phase := ""
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--phase" {
			phase = args[i+1]
		}
	}
	cfg := config.New("/opt/mirabilis/config")
	r := runtime.NewLocalRunner()
	switch phase {
	case "create":
		return provision.Create(ctx, r, cfg)
	case "start":
		return provision.Start(ctx, r, cfg)
	default:
		return fmt.Errorf("provision: unknown --phase %q (want create or start)", phase)
	}
}
