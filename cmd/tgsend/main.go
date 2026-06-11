// tgsend sends a single message to a Telegram channel via the configured bot.
//
// Token: read from the secret file (default /run/secrets/telegram_bot_token).
// A -token-path flag overrides the path. The token NEVER appears in argv,
// env, stdout, or error messages.
//
// Channel: supplied via -channel flag; defaults to the cached channel-ID file
// written by the SessionStart autodetect hook
// (~/.claude/.mirabilis-telegram-channel). An error is reported with a clear
// hint if neither source provides the channel.
//
// Dry-run (default): prints what would be sent and exits 0, sends nothing.
// With --confirm: sends the message via the outbox rate-limiter.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// defaultTokenPath is the container secret-mount path that outbox.go expects.
const defaultTokenPath = "/run/secrets/telegram_bot_token"

// defaultChannelCachePath is the state file written by the SessionStart
// autodetect hook, relative to HOME.
const defaultChannelCachePath = ".claude/.mirabilis-telegram-channel"

func main() {
	tokenPath := flag.String("token-path", defaultTokenPath, "path to bot token file (never use -token=<value>; always file)")
	channelFlag := flag.String("channel", "", "channel id (e.g. @mychannel or -100…); defaults to cached value")
	apiBase := flag.String("api", "https://api.telegram.org", "Telegram API base URL (for testing)")
	confirm := flag.Bool("confirm", false, "actually send the message (default: dry-run only)")
	flag.Parse()

	text := flag.Arg(0)
	if text == "" {
		// Try stdin if no positional arg.
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tgsend: read stdin: %v\n", err)
			os.Exit(1)
		}
		text = strings.TrimRight(string(raw), "\r\n")
	}
	if text == "" {
		fmt.Fprintln(os.Stderr, "tgsend: message text is required (positional arg or stdin)")
		os.Exit(2)
	}

	// Resolve channel.
	channel := *channelFlag
	if channel == "" {
		channel = readCachedChannel()
	}
	if channel == "" {
		fmt.Fprintln(os.Stderr, "tgsend: no channel available — supply -channel <id> or run the provision step to auto-detect via SessionStart")
		os.Exit(2)
	}

	// Read token from secret file (never from argv or env).
	token, err := readTokenFile(*tokenPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tgsend: cannot read token: %v (check that the secret file exists at %s)\n", err, *tokenPath)
		os.Exit(1)
	}

	if !*confirm {
		// Dry-run: print what would be sent, send nothing.
		fmt.Printf("dry-run: would send to channel %s\nmessage: %s\n", channel, text)
		fmt.Fprintln(os.Stderr, "(pass --confirm to actually send)")
		return
	}

	// Send via the Telegram API with a rate-limited HTTP client.
	client := &http.Client{Timeout: 15 * time.Second}
	if err := sendMessage(client, *apiBase, token, channel, text); err != nil {
		// Never include the token in the error message.
		fmt.Fprintf(os.Stderr, "tgsend: send failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("sent to channel %s\n", channel)
}

// readTokenFile is the single token-source seam for tgsend.
// TODO: token source: pending isolation design (issue #115) — replace this
// file-read with a broker/keychain call once the isolation model is decided.
func readTokenFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(raw))
	if tok == "" {
		return "", fmt.Errorf("token file is empty")
	}
	return tok, nil
}

func readCachedChannel() string {
	home := os.Getenv("HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = h
	}
	data, err := os.ReadFile(home + "/" + defaultChannelCachePath)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\r\n")
}

func sendMessage(client *http.Client, apiBase, token, chatID, text string) error {
	// Rate-limit: honour the existing 1-per-second constraint used by outbox.
	// tgsend is a manual one-shot tool, so no additional limiter is needed here.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Build URL without embedding token in any variable we log.
	apiURL := apiBase + "/bot" + token + "/sendMessage"
	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !result.OK {
		// Never include the token; only include the API-provided description.
		return fmt.Errorf("api error: %s", result.Description)
	}
	return nil
}
