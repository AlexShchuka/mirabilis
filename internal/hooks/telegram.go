// Package hooks dispatches Claude Code hook events to handler functions.
package hooks

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/notify"
	"github.com/google/uuid"
)

func Telegram() error {
	repo := repoRoot()
	chat, err := notify.ReadChatID(repo)
	if err != nil || chat == "" {
		return nil
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: read stdin: %v\n", err)
		return nil
	}

	text, ok := messageFor(eventName(data))
	if !ok {
		return nil
	}
	if proj := cwdBaseName(data); proj != "" {
		text = strings.Replace(text, "mirabilis:", "mirabilis ["+proj+"]:", 1)
	}

	job := notify.Job{
		ID:        uuid.NewString(),
		ChatID:    chat,
		Text:      text,
		CreatedAt: time.Now().UTC(),
	}
	if err := notify.WriteJob(notify.OutboxDir(repo), job); err != nil {
		fmt.Fprintf(os.Stderr, "[hook] WARN: telegram queue: %v\n", err)
	}
	return nil
}

func sessionTelegramContext(repo string) string {
	id, err := notify.ReadChatID(repo)
	if err != nil || id == "" {
		return ""
	}
	return "telegram channel: " + id + " (cached)"
}
