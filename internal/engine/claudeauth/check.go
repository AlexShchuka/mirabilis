package claudeauth

import (
	"context"
	"strings"
)

type tokenReader interface {
	Token(ctx context.Context) (string, error)
}

func Present(ctx context.Context, ts tokenReader) bool {
	token, err := ts.Token(ctx)
	return err == nil && strings.HasPrefix(token, tokenPrefix)
}
