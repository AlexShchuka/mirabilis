package notify

import (
	"context"
	"fmt"
	"sync"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

type Notifier interface {
	Send(ctx context.Context, chatID, text string) error
}

type Constructor func(store secrets.Store) (Notifier, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Constructor)
)

func Register(name string, ctor Constructor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = ctor
}

func New(name string, store secrets.Store) (Notifier, error) {
	registryMu.RLock()
	ctor, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("notify: unknown adapter %q", name)
	}
	return ctor(store)
}
