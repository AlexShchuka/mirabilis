package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
	"github.com/AlexShchuka/mirabilis/internal/engine/provision"
	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
	"github.com/AlexShchuka/mirabilis/internal/engine/status"
	"github.com/AlexShchuka/mirabilis/internal/hooks"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/app"
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
	if len(args) == 0 {
		return runTUI()
	}
	switch args[0] {
	case "-version", "--version", "version":
		fmt.Println(effectiveVersion())
		return nil
	case "-h", "-help", "--help", "help":
		fmt.Print("usage: mirabilis [command]\n\ncommands:\n  provision   run a provisioning phase inside the container\n  hook        dispatch a git hook by name\n  notify      host-side notification commands\n\nflags:\n  --version   print the build version and exit\n  --help      print this message and exit\n")
		return nil
	case "provision":
		return runProvision(context.Background(), args[1:])
	case "hook":
		if len(args) < 2 {
			return fmt.Errorf("hook: missing name argument")
		}
		return hooks.Dispatch(args[1])
	case "notify":
		return runNotify(context.Background(), args[1:])
	default:
		return fmt.Errorf("unknown argument %q — run 'mirabilis --help' for usage", args[0])
	}
}

func runTUI() error {
	repo := resolveRepo()
	if err := acquireFlock(repo); err != nil {
		fmt.Fprintln(os.Stderr, "mirabilis: already running")
		return nil
	}

	f, err := newFacade(repo)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	defer f.obs.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := f.proxy.Start(ctx); err != nil {
			f.obs.Logger("authproxy").Error("listen failed", "err", err)
		}
	}()
	status.New(f.docker, f.obs).Start(ctx)

	chatID, cerr := notify.ReadChatID(repo)
	if cerr == nil && chatID != "" {
		n := notify.NewTelegram(f.store, "")
		go notify.Watch(ctx, notify.OutboxDir(repo), n, f.obs, 0)
	}

	a := app.New(ctx, f)
	_, err = tea.NewProgram(a, tea.WithOutput(os.Stderr), tea.WithContext(ctx)).Run()
	return err
}

func runProvision(ctx context.Context, args []string) error {
	phase := ""
	proxyAddr := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--phase":
			if i+1 < len(args) {
				i++
				phase = args[i]
			}
		case "--proxy-addr":
			if i+1 < len(args) {
				i++
				proxyAddr = args[i]
			}
		}
	}

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("provision: read stdin: %w", err)
	}
	sessionKey := strings.TrimSpace(string(raw))

	home, _ := os.UserHomeDir()
	repo := resolveRepo()

	o, oerr := obs.New(logPathFor(repo))
	if oerr != nil {
		return fmt.Errorf("provision: obs: %w", oerr)
	}

	deps := provision.Deps{
		Runner:     newHost(),
		Cfg:        configFor(repo),
		Log:        o.Logger("provision"),
		Repo:       repo,
		Home:       home,
		ProxyAddr:  proxyAddr,
		SessionKey: sessionKey,
	}
	return provision.RunPhase(ctx, deps, phase)
}

func runNotify(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "send" {
		return fmt.Errorf("notify: unknown subcommand — use 'notify send'")
	}
	args = args[1:]

	repo := resolveRepo()

	chatID, _ := notify.ReadChatID(repo)
	var text string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--chat":
			if i+1 < len(args) {
				i++
				chatID = args[i]
			}
		default:
			text = strings.Join(args[i:], " ")
			i = len(args)
		}
	}
	if text == "" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("notify send: read stdin: %w", err)
		}
		text = strings.TrimSpace(string(raw))
	}
	if chatID == "" {
		return fmt.Errorf("notify send: chat-id not found — configure telegram first")
	}

	home, _ := os.UserHomeDir()
	store := newPlatformStore(repo, home)
	return notify.SendDirect(ctx, store, "", chatID, text)
}

func newPlatformStore(repo, home string) secrets.Store {
	return platformStore(repo, home)
}
