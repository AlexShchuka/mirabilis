package claudeauth

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

const (
	tokenKey    = "claude-token"
	tokenPrefix = "sk-ant-oat01-"
)

var errNotOATToken = errors.New("stored claude token is not an oat token")

type cachedSource struct {
	store secrets.Store

	mu    sync.Mutex
	token string
}

var _ TokenSource = (*cachedSource)(nil)

func NewSource(store secrets.Store) TokenSource {
	return &cachedSource{store: store}
}

func (s *cachedSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" {
		return s.token, nil
	}
	value, err := s.store.Get(ctx, tokenKey)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(value, tokenPrefix) {
		return "", errNotOATToken
	}
	s.token = value
	return value, nil
}

func (s *cachedSource) Invalidate() {
	s.mu.Lock()
	s.token = ""
	s.mu.Unlock()
}
