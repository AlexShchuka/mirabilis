package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

const (
	TokenKey = "telegram-token"

	defaultBaseURL = "https://api.telegram.org"
)

type Telegram struct {
	store   secrets.Store
	baseURL string
	client  *http.Client
}

var _ Notifier = (*Telegram)(nil)

func NewTelegram(store secrets.Store, baseURL string) *Telegram {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Telegram{
		store:   store,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *Telegram) Send(ctx context.Context, chatID, text string) error {
	token, err := t.store.Get(ctx, TokenKey)
	if err != nil {
		return fmt.Errorf("notify telegram: read token: %w", err)
	}
	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)
	body, err := t.postForm(ctx, token, "sendMessage", params)
	if err != nil {
		return err
	}
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("notify telegram: decode response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("notify telegram: api error: %s", result.Description)
	}
	return nil
}

func (t *Telegram) postForm(ctx context.Context, token, method string, params url.Values) ([]byte, error) {
	apiURL := t.baseURL + "/bot" + token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, redact(fmt.Errorf("notify telegram: build request: %w", err), token)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, redact(fmt.Errorf("notify telegram: http: %w", err), token)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, redact(fmt.Errorf("notify telegram: read body: %w", err), token)
	}
	return body, nil
}

func redact(err error, token string) error {
	if token == "" || !strings.Contains(err.Error(), token) {
		return err
	}
	return errors.New(strings.ReplaceAll(err.Error(), token, "<redacted>"))
}

func SendDirect(ctx context.Context, store secrets.Store, baseURL, chatID, text string) error {
	return NewTelegram(store, baseURL).Send(ctx, chatID, text)
}
