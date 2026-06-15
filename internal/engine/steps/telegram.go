package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

const telegramSkip = "skip"

type telegramStep struct {
	d       Deps
	baseURL string
}

func newTelegram(d Deps) *telegramStep {
	return &telegramStep{d: d}
}

const telegramConfigureTimeout = 45 * time.Second

func (s *telegramStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:     "telegram",
		Title:    "Telegram",
		Deps:     []string{configStepName},
		Kind:     pipeline.Interactive,
		Optional: true,
	}
}

func (s *telegramStep) Check(ctx context.Context) (bool, error) {
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
		return nil
	}
	out <- pipeline.Event{Kind: pipeline.EvLine, Step: "telegram", Line: "saving token…"}
	out <- pipeline.Event{Kind: pipeline.EvLine, Step: "telegram", Line: "detecting channel…"}
	cfgCtx, cfgCancel := context.WithTimeout(ctx, telegramConfigureTimeout)
	defer cfgCancel()
	return notify.Configure(cfgCtx, s.d.Store, s.baseURL, s.d.Repo, token)
}
