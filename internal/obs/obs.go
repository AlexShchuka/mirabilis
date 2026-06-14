// Package obs provides a thread-safe observable state map for named subsystem health.
package obs

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync"
)

type State int

const (
	StateUnknown State = iota
	StateOK
	StateDegraded
	StateOff
)

func (s State) String() string {
	switch s {
	case StateOK:
		return "ok"
	case StateDegraded:
		return "degraded"
	case StateOff:
		return "off"
	default:
		return "unknown"
	}
}

type NodeStatus struct {
	Detail string
	State  State
}

type Snapshot map[string]NodeStatus

type Obs struct {
	logger *slog.Logger
	file   *os.File

	mu       sync.Mutex
	statuses Snapshot
	watchers []chan Snapshot
}

func New(logPath string) (*Obs, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &Obs{
		logger:   slog.New(slog.NewTextHandler(f, nil)),
		file:     f,
		statuses: make(Snapshot),
	}, nil
}

func (o *Obs) Logger(node string) *slog.Logger {
	return o.logger.With(slog.String("node", node))
}

func (o *Obs) Set(node string, st State, detail string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.statuses[node] = NodeStatus{Detail: detail, State: st}
	for _, ch := range o.watchers {
		snap := maps.Clone(o.statuses)
		select {
		case ch <- snap:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- snap:
			default:
			}
		}
	}
}

func (o *Obs) Snapshot() Snapshot {
	o.mu.Lock()
	defer o.mu.Unlock()
	return maps.Clone(o.statuses)
}

func (o *Obs) Watch() <-chan Snapshot {
	ch := make(chan Snapshot, 1)
	o.mu.Lock()
	defer o.mu.Unlock()
	o.watchers = append(o.watchers, ch)
	return ch
}

func (o *Obs) Unwatch(ch <-chan Snapshot) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, w := range o.watchers {
		if w == ch {
			o.watchers = append(o.watchers[:i], o.watchers[i+1:]...)
			return
		}
	}
}

func (o *Obs) Close() error {
	return o.file.Close()
}
