package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type FileStore struct {
	dir string
}

var _ Store = (*FileStore)(nil)

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func (s *FileStore) Get(_ context.Context, key string) (string, error) {
	b, err := os.ReadFile(s.path(key))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (s *FileStore) Set(_ context.Context, key, value string) error {
	p := s.path(key)
	if err := os.WriteFile(p, []byte(value), 0o600); err != nil {
		return err
	}
	return os.Chmod(p, 0o600)
}

func (s *FileStore) path(key string) string {
	return filepath.Join(s.dir, fileName(key))
}
