package secrets

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const defaultKeychainTimeout = 5 * time.Second

type KeychainStore struct {
	runner        exec.Runner
	legacyFileDir string
	timeout       time.Duration
}

var _ Store = (*KeychainStore)(nil)

func NewKeychainStore(r exec.Runner, legacyFileDir string) *KeychainStore {
	return &KeychainStore{
		runner:        r,
		legacyFileDir: legacyFileDir,
		timeout:       defaultKeychainTimeout,
	}
}

func (s *KeychainStore) Get(ctx context.Context, key string) (string, error) {
	acct := keychainAccount()
	out, err := s.security(ctx, nil, "find-generic-password", "-a", acct, "-s", entryName(key), "-w")
	if err == nil {
		return strings.TrimSpace(out), nil
	}
	if isCtxErr(err) {
		return "", err
	}
	return s.migrate(ctx, acct, key)
}

func (s *KeychainStore) Set(ctx context.Context, key, value string) error {
	_, err := s.security(ctx, strings.NewReader(value),
		"add-generic-password", "-U", "-a", keychainAccount(), "-s", entryName(key), "-w")
	return err
}

func (s *KeychainStore) migrate(ctx context.Context, acct, key string) (string, error) {
	legacyEntry := entryName(key) + "-token"
	out, err := s.security(ctx, nil, "find-generic-password", "-a", acct, "-s", legacyEntry, "-w")
	if err == nil {
		value := strings.TrimSpace(out)
		if err := s.Set(ctx, key, value); err != nil {
			return "", err
		}
		_, _ = s.security(ctx, nil, "delete-generic-password", "-a", acct, "-s", legacyEntry)
		_ = os.Remove(s.legacyFilePath(key))
		return value, nil
	}
	if isCtxErr(err) {
		return "", err
	}
	b, err := os.ReadFile(s.legacyFilePath(key))
	if err != nil {
		return "", ErrNotFound
	}
	value := strings.TrimSpace(string(b))
	if err := s.Set(ctx, key, value); err != nil {
		return "", err
	}
	_ = os.Remove(s.legacyFilePath(key))
	return value, nil
}

func (s *KeychainStore) security(ctx context.Context, stdin io.Reader, args ...string) (string, error) {
	runCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	out, err := exec.Run(runCtx, s.runner, exec.Spec{Stdin: stdin, Argv: append([]string{"security"}, args...)})
	if err != nil {
		if cerr := runCtx.Err(); cerr != nil {
			return "", cerr
		}
		return "", err
	}
	return out, nil
}

func (s *KeychainStore) legacyFilePath(key string) string {
	return filepath.Join(s.legacyFileDir, fileName(key))
}

func entryName(key string) string {
	return "mirabilis-" + key
}

func keychainAccount() string {
	if a := os.Getenv("MIRABILIS_KEYCHAIN_ACCOUNT"); a != "" {
		return a
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "mirabilis"
}

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
