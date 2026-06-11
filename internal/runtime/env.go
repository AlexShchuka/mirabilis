package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

// blockedFromContainer lists env vars that must never reach the container.
// TELEGRAM_BOT_TOKEN is blocked because the token is held only on the host.
var blockedFromContainer = map[string]bool{
	"TELEGRAM_BOT_TOKEN": true,
}

func ComposeEnv(repo string) []string {
	// NOTE: TELEGRAM_BOT_TOKEN is intentionally NOT injected and is also
	// blocked from the pass-through so it can never reach the container,
	// even if it happens to be set in the host environment.
	// TELEGRAM_CHAT_ID is injected (not a secret — needed for queue routing).
	managed := map[string]string{
		"MIRABILIS_VERSION": GitShort(repo),
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

// KeychainGetTelegramChat returns the cached Telegram chat ID from the host
// keychain / env fallback. Returns empty string if not configured.
// This is not a secret (just a chat ID), so it is safe to log or use in URLs.
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
	// Fallback: host-side statefile written by the Telegram provision step.
	// Reads ~/.claude/.mirabilis-telegram-token (0600) without importing
	// the provision package (which would create an import cycle).
	if name == "telegram-token" {
		return readTelegramTokenFile()
	}
	return ""
}

// readTelegramTokenFile is the single token-source seam for runtime/env.
// TODO: token source: pending isolation design (issue #115) — replace this
// file-read with a broker/keychain call once the isolation model is decided.
func readTelegramTokenFile() string {
	home := os.Getenv("HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = h
	}
	path := filepath.Join(home, ".claude", ".mirabilis-telegram-token")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := strings.TrimRight(string(data), "\r\n")
	return s
}
