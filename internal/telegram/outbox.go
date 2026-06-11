package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const DefaultTokenPath = "/run/secrets/telegram_bot_token"

const maxRate = time.Second

type ConfirmFunc func(chatID, text string) bool

type Outbox struct {
	tokenPath     string
	allowedChatID string // pinned channel: the ONLY chat_id we will ever post to
	baseURL       string
	confirm       ConfirmFunc
	mu            sync.Mutex
	lastSent      time.Time
	client        *http.Client
}

// NewOutbox creates an Outbox pinned to allowedChatID.
// allowedChatID is the ONLY chat_id that Send() will ever post to; if it is
// empty, ALL sends are refused (fail closed). This is the single source of
// truth for the least-privilege channel constraint.
//
// honesty note: this pin binds our code path only — a raw token used directly
// (e.g., curl) still reaches any chat the bot is in. The pin is defense-in-
// depth combined with: the bot being a member of exactly one channel
// (Telegram-enforced) and the pending token-isolation work (issue #115).
func NewOutbox(tokenPath, allowedChatID string, confirm ConfirmFunc) (*Outbox, error) {
	if confirm == nil {
		return nil, fmt.Errorf("telegram outbox: confirm callback is required")
	}
	if tokenPath == "" {
		tokenPath = DefaultTokenPath
	}
	return &Outbox{
		tokenPath:     tokenPath,
		allowedChatID: allowedChatID,
		baseURL:       "https://api.telegram.org",
		confirm:       confirm,
		client:        &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (o *Outbox) Send(ctx context.Context, chatID, text string) error {
	// Channel-pin check: fail closed BEFORE confirm or any network call.
	// An empty allowedChatID means no channel has been configured — refuse all.
	if o.allowedChatID == "" {
		return fmt.Errorf("telegram outbox: no pinned channel configured — call NewOutbox with a non-empty allowedChatID")
	}
	if chatID != o.allowedChatID {
		return fmt.Errorf("telegram outbox: refused chat_id %q — only the pinned channel %q is allowed", chatID, o.allowedChatID)
	}

	if !o.confirm(chatID, text) {
		return fmt.Errorf("telegram outbox: send not confirmed")
	}

	token, err := o.readToken()
	if err != nil {
		return fmt.Errorf("telegram outbox: cannot read token: %w", err)
	}

	o.mu.Lock()
	elapsed := time.Since(o.lastSent)
	if elapsed < maxRate {
		o.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(maxRate - elapsed):
		}
		o.mu.Lock()
	}
	o.lastSent = time.Now()
	o.mu.Unlock()

	return o.doSend(ctx, token, chatID, text)
}

func (o *Outbox) readToken() (string, error) {
	raw, err := os.ReadFile(o.tokenPath)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(raw))
	if tok == "" {
		return "", fmt.Errorf("token file is empty")
	}
	return tok, nil
}

func (o *Outbox) doSend(ctx context.Context, token, chatID, text string) error {
	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", o.baseURL, token)
	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("telegram outbox: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram outbox: http: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("telegram outbox: decode response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram outbox: api error: %s", result.Description)
	}
	return nil
}

func (o *Outbox) withBaseURL(u string) *Outbox {
	o.baseURL = u
	return o
}
