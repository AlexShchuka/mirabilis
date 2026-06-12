package claudeauth

import (
	"context"
	"io"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

const (
	obsNode      = "claude-auth"
	storeTimeout = 10 * time.Second
	tailWindow   = 4096
)

var tokenPattern = regexp.MustCompile(`sk-ant-oat01-[A-Za-z0-9_-]+`)

func SetupArgv() []string {
	return []string{"claude", "setup-token"}
}

type Extractor struct {
	mu    sync.Mutex
	out   io.Writer
	tail  []byte
	token string
}

var _ io.Writer = (*Extractor)(nil)

func NewExtractor() *Extractor {
	return &Extractor{}
}

func (e *Extractor) Tee(w io.Writer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.out = w
}

func (e *Extractor) Write(p []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tail = append(e.tail, p...)
	if matches := tokenPattern.FindAll(e.tail, -1); len(matches) > 0 {
		e.token = string(matches[len(matches)-1])
	}
	if len(e.tail) > tailWindow {
		copy(e.tail, e.tail[len(e.tail)-tailWindow:])
		e.tail = e.tail[:tailWindow]
	}
	if e.out == nil {
		return len(p), nil
	}
	return e.out.Write(p)
}

func (e *Extractor) Token() (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.token, e.token != ""
}

func StoreInBackground(store secrets.Store, token string, o *obs.Obs) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		defer cancel()
		log := o.Logger("claudeauth")
		if err := store.Set(ctx, tokenKey, token); err != nil {
			log.Error("token write failed", slog.Any("error", err))
			o.Set(obsNode, obs.StateDegraded, "token write failed")
			return
		}
		log.Info("token stored")
		o.Set(obsNode, obs.StateOK, "token stored")
	}()
}
