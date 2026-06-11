package runtime

import (
	"os"
	"os/exec"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/tgtoken"
)

var blockedFromContainer = map[string]bool{
	"TELEGRAM_BOT_TOKEN": true,
}

func ComposeEnv(repo string) []string {
	managed := map[string]string{
		"MIRABILIS_VERSION": GitShort(repo),
		"MIRABILIS_REPO":    repo,
		"TELEGRAM_CHAT_ID":  keychainGet("telegram-chat"),
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
			if blockedFromContainer[k] {
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

func KeychainGetTelegramChat() string {
	return keychainGet("telegram-chat")
}

func keychainGet(name string) string {
	if val, ok := keychainLookup(name); ok {
		return val
	}
	if env := keychainEnv(name); env != "" {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	if name == "telegram-token" {
		return tgtoken.Read()
	}
	return ""
}
