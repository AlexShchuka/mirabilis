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

type CachedSource struct {
	store secrets.Store

	mu    sync.Mutex
	token string
}

func NewSource(store secrets.Store) *CachedSource {
	return &CachedSource{store: store}
}

func (s *CachedSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.token != "" {
		v := s.token
		s.mu.Unlock()
		return v, nil
	}
	s.mu.Unlock()

	value, err := s.store.Get(ctx, tokenKey)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(value, tokenPrefix) {
		return "", errNotOATToken
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" {
		return s.token, nil
	}
	s.token = value
	return value, nil
}

func (s *CachedSource) Invalidate() {
	s.mu.Lock()
	s.token = ""
	s.mu.Unlock()
}

type staticSource struct {
	token string
}

func NewStaticSource(token string) *staticSource {
	return &staticSource{token: token}
}

func (s *staticSource) Token(context.Context) (string, error) {
	if s.token == "" {
		return "", errors.New("claudeauth: static source has no token")
	}
	return s.token, nil
}
