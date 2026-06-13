package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

	owner := true
	if err := acquireFlock(repo); err != nil {
		if !errors.Is(err, errFlockHeld) {
			return fmt.Errorf("init: %w", err)
		}
		owner = false
	}
	defer releaseFlock()

	f, err := newFacade(repo)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	defer f.obs.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	go func() {
		<-ctx.Done()
		stop()
	}()

	status.New(f.docker, f.obs).Start(ctx)

	if owner {
		startSession(ctx, f, repo, "")
	}

	a := app.New(ctx, f, !owner)
	p := tea.NewProgram(a, tea.WithOutput(os.Stderr), tea.WithContext(ctx))

	if !owner {
		go promoteLoop(ctx, lockPathFor(repo), promoteInterval, f.obs.Logger("promote"), func(lock *os.File) {
			setFlock(lock)
			startSession(ctx, f, repo, promotedKey(f, repo))
			p.Send(app.PromotedMsg())
		})
	}

	_, err = p.Run()
	return err
}

func startSession(ctx context.Context, f *facade, repo, key string) {
	proxy := f.newProxy(key)
	if err := writeSessionKey(repo, proxy.Key()); err != nil {
		f.obs.Logger("authproxy").Error("session-key persist failed", "err", err)
	}
	go func() {
		if err := proxy.Start(ctx); err != nil {
			f.obs.Logger("authproxy").Error("listen failed", "err", err)
		}
	}()
	go notify.Watch(ctx, notify.OutboxDir(repo), notify.NewTelegram(f.store, ""), f.obs, 0)
}

func promotedKey(f *facade, repo string) string {
	key, err := readSessionKey(repo)
	if err != nil || key == "" {
		f.obs.Logger("authproxy").Error("session-key missing on promotion; generating fresh", "err", err)
		return ""
	}
	return key
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
	defer o.Close()

	deps := provision.Deps{
		Runner:      newHost(),
		Cfg:         configFor(repo),
		Log:         o.Logger("provision"),
		Repo:        repo,
		Home:        home,
		ProxyAddr:   proxyAddr,
		SessionKey:  sessionKey,
		Fingerprint: os.Getenv("MIRABILIS_VERSION"),
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
