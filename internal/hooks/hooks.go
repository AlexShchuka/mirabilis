package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func eventName(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var v struct {
		HookEventName string `json:"hook_event_name"`
	}
	if err := json.Unmarshal(data, &v); err == nil && v.HookEventName != "" {
		return v.HookEventName
	}
	s := string(data)
	if strings.Contains(s, `"hook_event_name"`) {
		if strings.Contains(s, `"Notification"`) {
			return "Notification"
		}
		if strings.Contains(s, `"Stop"`) {
			return "Stop"
		}
	}
	return ""
}

func messageFor(event string) (string, bool) {
	switch event {
	case "Notification":
		return "❓ mirabilis: needs your input", true
	case "Stop":
		return "✅ mirabilis: task finished", true
	}
	return "", false
}

func Telegram() error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chat := os.Getenv("TELEGRAM_CHAT_ID")
	if token == "" || chat == "" {
		return nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil
	}

	name := eventName(data)
	text, ok := messageFor(name)
	if !ok {
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	endpoint := "https://api.telegram.org/bot" + token + "/sendMessage"
	form := url.Values{}
	form.Set("chat_id", chat)
	form.Set("text", text)
	resp, err := client.PostForm(endpoint, form)
	if err != nil {
		return nil
	}
	resp.Body.Close()
	return nil
}

func Dispatch(name string) error {
	switch name {
	case "telegram":
		return Telegram()
	default:
		return fmt.Errorf("unknown hook: %s", name)
	}
}
