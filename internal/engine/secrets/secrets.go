package secrets

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("secrets: secret not found")

type Store interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

func fileName(key string) string {
	return ".mirabilis-" + key
}
