package tgtoken

import (
	"os"
	"path/filepath"
	"strings"
)

const Filename = ".mirabilis-telegram-token"

func Read() string {
	home := os.Getenv("HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = h
	}
	path := filepath.Join(home, ".claude", Filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\r\n")
}
