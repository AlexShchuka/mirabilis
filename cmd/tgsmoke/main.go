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

func main() {
	tokenAPath := flag.String("token-a", "", "path to bot A token file")
	tokenDPath := flag.String("token-d", "", "path to bot D token file")
	channelID := flag.String("channel", "", "channel id (e.g. @mychannel or -100...)")
	baseURL := flag.String("api", "https://api.telegram.org", "Telegram API base URL (for testing)")
	flag.Parse()

	if *tokenAPath == "" || *tokenDPath == "" || *channelID == "" {
		fmt.Fprintln(os.Stderr, "usage: tgsmoke -token-a <file> -token-d <file> -channel <id>")
		os.Exit(2)
	}

	tokenA, err := readToken(*tokenAPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: read token-a: %v\n", err)
		os.Exit(1)
	}
	tokenD, err := readToken(*tokenDPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: read token-d: %v\n", err)
		os.Exit(1)
	}

	canary := fmt.Sprintf("tgsmoke-canary-%d", time.Now().UnixNano())
	client := &http.Client{Timeout: 15 * time.Second}

	if err := sendMessage(client, *baseURL, tokenA, *channelID, canary); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: bot A sendMessage: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("bot A: canary posted")

	time.Sleep(2 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	found, err := pollForCanary(ctx, client, *baseURL, tokenD, canary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: bot D getUpdates: %v\n", err)
		os.Exit(1)
	}
	if !found {
		fmt.Println("FAIL: canary not seen by bot D within timeout")
		os.Exit(1)
	}
	fmt.Println("PASS: bot D received the canary channel_post")
}

func readToken(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(raw))
	if tok == "" {
		return "", fmt.Errorf("token file %s is empty", path)
	}
	return tok, nil
}

func sendMessage(client *http.Client, baseURL, token, chatID, text string) error {
	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, token)
	params := url.Values{}
	params.Set("chat_id", chatID)
	params.Set("text", text)

	resp, err := client.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("api error: %s", result.Description)
	}
	return nil
}

func pollForCanary(ctx context.Context, client *http.Client, baseURL, token, canary string) (bool, error) {
	offset := 0
	for {
		select {
		case <-ctx.Done():
			return false, nil
		default:
		}

		msgs, newOffset, err := getUpdates(client, baseURL, token, offset)
		if err != nil {
			return false, err
		}
		offset = newOffset

		for _, m := range msgs {
			if m == canary {
				return true, nil
			}
		}

		select {
		case <-ctx.Done():
			return false, nil
		case <-time.After(1 * time.Second):
		}
	}
}

func getUpdates(client *http.Client, baseURL, token string, offset int) ([]string, int, error) {
	apiURL := fmt.Sprintf("%s/bot%s/getUpdates", baseURL, token)
	params := url.Values{}
	params.Set("allowed_updates", `["channel_post"]`)
	params.Set("timeout", "0")
	if offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", offset))
	}

	resp, err := client.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, offset, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		OK     bool `json:"ok"`
		Result []struct {
			UpdateID    int `json:"update_id"`
			ChannelPost *struct {
				Text string `json:"text"`
			} `json:"channel_post"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, offset, fmt.Errorf("decode: %w", err)
	}
	if !result.OK {
		return nil, offset, fmt.Errorf("api returned ok=false")
	}

	newOffset := offset
	var texts []string
	for _, u := range result.Result {
		if u.UpdateID >= newOffset {
			newOffset = u.UpdateID + 1
		}
		if u.ChannelPost != nil {
			texts = append(texts, u.ChannelPost.Text)
		}
	}
	return texts, newOffset, nil
}
