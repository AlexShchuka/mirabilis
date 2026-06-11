package tgtoken

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	Filename       = ".mirabilis-telegram-token"
	FilenameClaude = ".mirabilis-claude-token"
)

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func ReadFile(filename string) string {
	home := homeDir()
	if home == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", filename))
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\r\n")
}

func Read() string { return ReadFile(Filename) }
