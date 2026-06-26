// Package main is the mirabilis CLI entry point.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/AlexShchuka/mirabilis/internal/engine/authproxy"
	"github.com/AlexShchuka/mirabilis/internal/engine/claudeauth"
	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
	"github.com/AlexShchuka/mirabilis/internal/engine/steps"
	"github.com/AlexShchuka/mirabilis/internal/obs"
	"github.com/AlexShchuka/mirabilis/internal/tui/app"
)

type facade struct {
	obs    *obs.Obs
	runner exec.Runner
	docker sandbox.Docker
	sb     *sandbox.Sandbox
	store  secrets.Store
	tokens authproxy.TokenSource
	deps   steps.Deps
	repo   string
	port   int
	mu     sync.RWMutex
	key    string
}

var _ app.Facade = (*facade)(nil)

func newFacade(repo string) (*facade, error) {
	notify.Register("telegram", func(store secrets.Store) (notify.Notifier, error) {
		return notify.NewTelegram(store, ""), nil
	})

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("facade: home dir: %w", err)
	}

	o, err := obs.New(logPathFor(repo))
	if err != nil {
		return nil, fmt.Errorf("facade: obs: %w", err)
	}

	runner := newHost()

	docker, err := sandbox.NewMoby()
	if err != nil {
		return nil, fmt.Errorf("facade: docker client: %w", err)
	}

	sb := sandbox.New(runner, docker, repo)

	store := platformStore(repo, home)

	tokens := claudeauth.NewSource(store)

	port := config.AuthProxyPort(repo)

	proxyAddrFn := func() string {
		return "http://host.docker.internal:" + strconv.Itoa(port)
	}

	f := &facade{
		obs:    o,
		runner: runner,
		docker: docker,
		sb:     sb,
		store:  store,
		tokens: tokens,
		repo:   repo,
		port:   port,
	}

	f.deps = steps.Deps{
		Runner:     runner,
		Docker:     docker,
		Sandbox:    sb,
		Store:      store,
		Tokens:     tokens,
		Obs:        o,
		ProxyAddr:  proxyAddrFn,
		SessionKey: f.sessionKey,
		Repo:       repo,
	}

	return f, nil
}

func (f *facade) sessionKey() string {
	f.mu.RLock()
	k := f.key
	f.mu.RUnlock()
	if k != "" {
		return k
	}
	k, _ = readSessionKey(f.repo)
	return k
}

func (f *facade) newProxy(key string) *authproxy.Proxy {
	p := authproxy.New(f.tokens, f.obs, f.port, key)
	f.mu.Lock()
	f.key = p.Key()
	f.mu.Unlock()
	return p
}

func (f *facade) Loadouts() []app.LoadoutChoice {
	names := config.ReadLoadoutCatalog(f.repo)
	if len(names) == 0 {
		return nil
	}
	active, ok := config.ReadLoadout(f.repo)
	if !ok || active == "" {
		active = config.DefaultLoadout
	}
	out := make([]app.LoadoutChoice, 0, len(names))
	for _, name := range names {
		lo, ok := config.ReadLoadoutManifest(f.repo, name)
		if !ok {
			continue
		}
		out = append(out, app.LoadoutChoice{
			Key:     name,
			Effort:  lo.Effort,
			Harness: true,
			Batch:   lo.Batch,
			Default: name == active,
		})
	}
	return out
}

func (f *facade) SelectLoadout(name string) error {
	return config.WriteLoadout(f.repo, name)
}

func (f *facade) LaunchSteps() []pipeline.Command {
	if config.LaunchBatched(f.repo) {
		return steps.LaunchBatched(f.deps)
	}
	return steps.Launch(f.deps)
}

func (f *facade) Version() string {
	return effectiveVersion()
}

func (f *facade) Logger() *slog.Logger {
	return f.obs.Logger("app")
}

func (f *facade) StatusUpdates() <-chan obs.Snapshot {
	return f.obs.Watch()
}

func (f *facade) OnTokenExtracted(token string) {
	claudeauth.StoreInBackground(f.store, token, f.obs)
}

func (f *facade) NewTokenTee() (io.Writer, func() (string, bool)) {
	e := claudeauth.NewExtractor()
	return e, e.Token
}

func (f *facade) OpenVSCode(ctx context.Context) error {
	return f.sb.OpenVSCode(ctx)
}

func (f *facade) UpdateEcosystem(ctx context.Context) error {
	return steps.UpdateEcosystem(ctx, f.deps)
}

func (f *facade) OpenURL(ctx context.Context, url string) error {
	return sandbox.OpenURL(ctx, f.runner, url)
}

func (f *facade) CopyText(ctx context.Context, text string) error {
	return sandbox.CopyText(ctx, f.runner, text)
}

func resolveRepo() string {
	if r := os.Getenv("MIRABILIS_REPO"); r != "" {
		return r
	}
	exe, err := os.Executable()
	if err == nil {
		if resolved, e := filepath.EvalSymlinks(exe); e == nil {
			exe = resolved
		}
		return filepath.Clean(filepath.Join(filepath.Dir(exe), ".."))
	}
	wd, _ := os.Getwd()
	return wd
}

func newHost() *exec.Host {
	return exec.NewHost()
}

func configFor(repo string) config.Config {
	return config.New(filepath.Join(repo, "config"))
}

func logPathFor(repo string) string {
	return config.LogPath(repo)
}
