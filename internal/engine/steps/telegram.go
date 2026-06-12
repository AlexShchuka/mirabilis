package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const (
	telegramEnvKey    = "TELEGRAM"
	telegramSkip      = "skip"
	tokenStoreTimeout = 10 * time.Second
)

type telegramStep struct {
	d       Deps
	baseURL string
}

func newTelegram(d Deps) *telegramStep {
	return &telegramStep{d: d}
}

func (s *telegramStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     "telegram",
		Title:    "Telegram",
		Deps:     []string{"stacks"},
		Kind:     pipeline.Interactive,
		Optional: true,
	}
}

func (s *telegramStep) Check(ctx context.Context) (bool, error) {
	if v, ok := dotenvRead(s.d.Repo, telegramEnvKey); ok && v == telegramSkip {
		return true, nil
	}
	if _, err := s.d.Store.Get(ctx, notify.TokenKey); err != nil {
		return false, nil
	}
	_, err := notify.ReadChatID(s.d.Repo)
	return err == nil, nil
}

func (s *telegramStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: "telegram", Payload: TelegramSetup{}}
	r, err := awaitResume(ctx, in)
	if err != nil {
		return err
	}
	token, ok := r.Value.(string)
	if !ok {
		return fmt.Errorf("steps: telegram: expected string result, got %T", r.Value)
	}
	if token == telegramSkip {
		return dotenvWrite(s.d.Repo, telegramEnvKey, telegramSkip)
	}
	if err := s.storeToken(ctx, token); err != nil {
		return err
	}
	chatID, err := notify.DetectChatID(ctx, s.d.Store, s.baseURL)
	if err != nil {
		return err
	}
	return notify.WriteChatID(s.d.Repo, chatID)
}

func (s *telegramStep) storeToken(ctx context.Context, token string) error {
	done := make(chan error, 1)
	go func() {
		sctx, cancel := context.WithTimeout(context.Background(), tokenStoreTimeout)
		defer cancel()
		done <- s.d.Store.Set(sctx, notify.TokenKey, token)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
