package telegram

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Message struct {
	MessageID int    `json:"message_id"`
	Date      int64  `json:"date"`
	TextHash  string `json:"sha256"`
	Text      string `json:"text"`
}

type Inbox struct {
	tokenPath  string
	baseURL    string
	mirrorPath string
	client     *http.Client
}

func NewInbox(tokenPath, mirrorPath string) (*Inbox, error) {
	if mirrorPath == "" {
		return nil, fmt.Errorf("telegram inbox: mirrorPath is required")
	}
	if tokenPath == "" {
		tokenPath = DefaultTokenPath
	}
	return &Inbox{
		tokenPath:  tokenPath,
		baseURL:    "https://api.telegram.org",
		mirrorPath: mirrorPath,
		client:     &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (in *Inbox) Poll(ctx context.Context, chatID string, offset int) ([]Message, int, error) {
	token, err := in.readToken()
	if err != nil {
		return nil, offset, fmt.Errorf("telegram inbox: cannot read token: %w", err)
	}

	apiURL := fmt.Sprintf("%s/bot%s/getUpdates", in.baseURL, token)
	params := url.Values{}
	params.Set("allowed_updates", `["channel_post"]`)
	params.Set("timeout", "0")
	if offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", offset))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, offset, fmt.Errorf("telegram inbox: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := in.client.Do(req)
	if err != nil {
		return nil, offset, fmt.Errorf("telegram inbox: http: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID    int `json:"update_id"`
			ChannelPost *struct {
				MessageID int    `json:"message_id"`
				Date      int64  `json:"date"`
				Text      string `json:"text"`
			} `json:"channel_post"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, offset, fmt.Errorf("telegram inbox: decode response: %w", err)
	}
	if !result.OK {
		return nil, offset, fmt.Errorf("telegram inbox: api returned ok=false")
	}

	var msgs []Message
	newOffset := offset
	for _, u := range result.Result {
		if u.UpdateID >= newOffset {
			newOffset = u.UpdateID + 1
		}
		if u.ChannelPost == nil {
			continue
		}
		sum := sha256.Sum256([]byte(u.ChannelPost.Text))
		msgs = append(msgs, Message{
			MessageID: u.ChannelPost.MessageID,
			Date:      u.ChannelPost.Date,
			TextHash:  fmt.Sprintf("%x", sum),
			Text:      u.ChannelPost.Text,
		})
	}
	return msgs, newOffset, nil
}

func (in *Inbox) Append(msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	f, err := os.OpenFile(in.mirrorPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("telegram inbox: open mirror: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, m := range msgs {
		if err := enc.Encode(m); err != nil {
			return fmt.Errorf("telegram inbox: write mirror: %w", err)
		}
	}
	return nil
}

func (in *Inbox) readToken() (string, error) {
	raw, err := os.ReadFile(in.tokenPath)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(raw))
	if tok == "" {
		return "", fmt.Errorf("token file is empty")
	}
	return tok, nil
}

func (in *Inbox) withBaseURL(u string) *Inbox {
	in.baseURL = u
	return in
}
