package notify

import (
	"context"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

const tokenStoreTimeout = 10 * time.Second

func Configure(ctx context.Context, store secrets.Store, baseURL, repo, token string) error {
	if err := storeToken(ctx, store, token); err != nil {
		return err
	}
	chatID, err := DetectChatID(ctx, store, baseURL)
	if err != nil {
		return err
	}
	return WriteChatID(repo, chatID)
}

func storeToken(ctx context.Context, store secrets.Store, token string) error {
	done := make(chan error, 1)
	go func() {
		sctx, cancel := context.WithTimeout(context.Background(), tokenStoreTimeout)
		defer cancel()
		done <- store.Set(sctx, TokenKey, token)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
