package claudeauth

import (
	"context"
	"strings"
)

func Present(ctx context.Context, ts TokenSource) bool {
	token, err := ts.Token(ctx)
	return err == nil && strings.HasPrefix(token, tokenPrefix)
}
