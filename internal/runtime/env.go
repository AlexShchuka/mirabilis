package runtime

import (
	"os"
	"os/exec"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func ComposeEnv(repo string) []string {
	managed := map[string]string{
		"MIRABILIS_VERSION":  GitShort(repo),
		"TELEGRAM_BOT_TOKEN": keychainGet("telegram-token"),
		"TELEGRAM_CHAT_ID":   keychainGet("telegram-chat"),
	}
	if stacks, ok := config.ReadStacks(repo); ok {
		managed["STACKS"] = stacks
	}
	var env []string
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok {
			if _, owned := managed[k]; owned {
				continue
			}
		}
		env = append(env, kv)
	}
	for k, v := range managed {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func GitShort(repo string) string {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func keychainEnv(name string) string {
	switch name {
	case "telegram-token":
		return "TELEGRAM_BOT_TOKEN"
	case "telegram-chat":
		return "TELEGRAM_CHAT_ID"
	}
	return ""
}

func keychainGet(name string) string {
	if val, ok := keychainLookup(name); ok {
		return val
	}
	return os.Getenv(keychainEnv(name))
}
