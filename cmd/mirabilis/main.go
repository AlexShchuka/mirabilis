package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/localllm"
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
		fmt.Print("usage: mirabilis [command]\n\ncommands:\n  serve       run proxy and notify services (managed automatically by TUI)\n  provision   run a provisioning phase inside the container\n  hook        dispatch a git hook by name\n  notify      host-side notification commands\n\nflags:\n  --version   print the build version and exit\n  --help      print this message and exit\n")
		return nil
	case "serve":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		defer stop()
		return runServe(ctx, resolveRepo())
	case "provision":
		return runProvision(context.Background(), args[1:])
	case "hook":
		if len(args) < 2 {
			return fmt.Errorf("hook: missing name argument")
		}
		return hooks.Dispatch(args[1])
	case "notify":
		return runNotify(context.Background(), args[1:])
	case "localllm":
		if len(args) < 2 || args[1] != "serve" {
			return fmt.Errorf("localllm: unknown subcommand — use 'localllm serve'")
		}
		return runLocalLLMServe(context.Background())
	default:
		return fmt.Errorf("unknown argument %q — run 'mirabilis --help' for usage", args[0])
	}
}

func runTUI() error {
	repo := resolveRepo()

	ensureServe(repo)
	waitForSessionKey(repo, 2*time.Second, 50*time.Millisecond)

	cleanup, err := registerClient(repo)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	defer cleanup()

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

	a := app.New(ctx, f)
	p := tea.NewProgram(a, tea.WithOutput(os.Stderr), tea.WithContext(ctx))

	_, err = p.Run()
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

func runLocalLLMServe(ctx context.Context) error {
	timeout := config.LocalLLMEffectiveTimeout()
	client := &http.Client{Timeout: timeout}
	baseURL := config.LocalLLMEffectiveBaseURL()
	model := config.LocalLLMEffectiveModel()
	if model == "" || model == "auto" {
		discCtx, discCancel := context.WithTimeout(ctx, timeout)
		defer discCancel()
		discovered, err := localllm.DiscoverModel(discCtx, baseURL, client)
		if err != nil {
			return fmt.Errorf("localllm: model discovery failed: %w", err)
		}
		model = discovered
	}
	c := &localllm.HTTPAdapter{
		BaseURL:   baseURL,
		Model:     model,
		Timeout:   timeout,
		MaxTokens: config.LocalLLMEffectiveMaxTokens(),
		Client:    client,
	}
	return localllm.ServeStdio(ctx, c)
}
