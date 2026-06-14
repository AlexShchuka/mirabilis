// Package notify delivers asynchronous messages via a file-backed queue and Telegram sender.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

var (
	ErrNoChannel       = errors.New("notify telegram: channel not detected — post a fresh message in the channel and retry")
	ErrWebhookConflict = errors.New("notify telegram: webhook active — delete it first with deleteWebhook")
)

func ChatIDPath(repo string) string {
	return filepath.Join(repo, ".mirabilis", "chat-id")
}

func ReadChatID(repo string) (string, error) {
	data, err := os.ReadFile(ChatIDPath(repo))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func WriteChatID(repo, id string) error {
	path := ChatIDPath(repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("notify: mkdir for chat-id: %w", err)
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(id)+"\n"), 0o644)
}

func DetectChatID(ctx context.Context, store secrets.Store, baseURL string) (string, error) {
	t := NewTelegram(store, baseURL)
	token, err := store.Get(ctx, TokenKey)
	if err != nil {
		return "", fmt.Errorf("notify telegram: read token: %w", err)
	}
	params := url.Values{}
	params.Set("allowed_updates", `["channel_post","my_chat_member","message"]`)
	params.Set("limit", "100")
	params.Set("timeout", "0")
	body, err := t.postForm(ctx, token, "getUpdates", params)
	if err != nil {
		return "", err
	}
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      []struct {
			ChannelPost  *chatHolder `json:"channel_post"`
			MyChatMember *chatHolder `json:"my_chat_member"`
			Message      *chatHolder `json:"message"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", ErrNoChannel
	}
	if !result.OK {
		if strings.Contains(result.Description, "webhook") {
			return "", ErrWebhookConflict
		}
		return "", ErrNoChannel
	}
	for _, u := range result.Result {
		if id := firstChannelID(u.ChannelPost, u.MyChatMember, u.Message); id != 0 {
			return strconv.FormatInt(id, 10), nil
		}
	}
	return "", ErrNoChannel
}

type chatHolder struct {
	Chat struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
}

func firstChannelID(holders ...*chatHolder) int64 {
	for _, h := range holders {
		if h != nil && h.Chat.Type == "channel" && h.Chat.ID != 0 {
			return h.Chat.ID
		}
	}
	return 0
}
