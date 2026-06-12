package secrets

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const (
	defaultKeychainTimeout = 5 * time.Second
	b64Prefix              = "mirabilis-b64:"
)

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
	raw, err := s.securityFind(ctx, acct, entryName(key))
	if err == nil {
		return decodeValue(raw), nil
	}
	if isCtxErr(err) {
		return "", err
	}
	return s.migrate(ctx, acct, key)
}

func (s *KeychainStore) Set(ctx context.Context, key, value string) error {
	acct := keychainAccount()
	entry := entryName(key)
	if strings.ContainsRune(acct, '\'') {
		return fmt.Errorf("keychain: account %q contains single quote", acct)
	}
	if strings.ContainsRune(entry, '\'') {
		return fmt.Errorf("keychain: entry %q contains single quote", entry)
	}
	stored := b64Prefix + base64.StdEncoding.EncodeToString([]byte(value))
	cmdline := "add-generic-password -U -a '" + acct + "' -s '" + entry + "' -w '" + stored + "'\n"
	if _, err := s.security(ctx, strings.NewReader(cmdline), "-i"); err != nil {
		return fmt.Errorf("keychain: set %q: %w", key, err)
	}
	got, err := s.securityFind(ctx, acct, entry)
	if err != nil {
		return fmt.Errorf("keychain: set %q: readback failed: %w", key, err)
	}
	if got == "" {
		return fmt.Errorf("keychain: set %q: readback returned empty", key)
	}
	if decodeValue(got) != value {
		return fmt.Errorf("keychain: set %q: readback mismatch", key)
	}
	return nil
}

func (s *KeychainStore) migrate(ctx context.Context, acct, key string) (string, error) {
	legacyEntry := entryName(key) + "-token"
	raw, err := s.securityFind(ctx, acct, legacyEntry)
	if err == nil {
		value := decodeValue(raw)
		if serr := s.Set(ctx, key, value); serr != nil {
			return "", serr
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
	if serr := s.Set(ctx, key, value); serr != nil {
		return "", serr
	}
	_ = os.Remove(s.legacyFilePath(key))
	return value, nil
}

func (s *KeychainStore) securityFind(ctx context.Context, acct, entry string) (string, error) {
	out, err := s.security(ctx, nil, "find-generic-password", "-a", acct, "-s", entry, "-w")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
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
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "mirabilis"
}

func decodeValue(raw string) string {
	if !strings.HasPrefix(raw, b64Prefix) {
		return raw
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(raw, b64Prefix))
	if err != nil {
		return raw
	}
	return string(decoded)
}

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
