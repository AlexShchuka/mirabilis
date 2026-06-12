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

var ErrNoChannel = errors.New("notify telegram: channel not detected — post anything in the channel")

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
	params.Set("allowed_updates", `["channel_post"]`)
	params.Set("limit", "1")
	params.Set("timeout", "0")
	body, err := t.postForm(ctx, token, "getUpdates", params)
	if err != nil {
		return "", err
	}
	var result struct {
		OK     bool `json:"ok"`
		Result []struct {
			ChannelPost *struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"channel_post"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil || !result.OK {
		return "", ErrNoChannel
	}
	for _, u := range result.Result {
		if u.ChannelPost == nil || u.ChannelPost.Chat.ID == 0 {
			continue
		}
		return strconv.FormatInt(u.ChannelPost.Chat.ID, 10), nil
	}
	return "", ErrNoChannel
}
