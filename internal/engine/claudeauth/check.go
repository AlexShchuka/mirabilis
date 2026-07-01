// Package claudeauth manages caching and validation of the Anthropic API token on the host side.
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
      return err == nil && token != ""
}
