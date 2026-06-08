package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

func composeEnv(repo string) []string {
	managed := map[string]string{
		"MIRABILIS_VERSION":  gitShort(repo),
		"TELEGRAM_BOT_TOKEN": keychainGet("telegram-token"),
		"TELEGRAM_CHAT_ID":   keychainGet("telegram-chat"),
	}
	if stacks, ok := readStacks(repo); ok {
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

var (
	gitShortOnce sync.Once
	gitShortVal  string
)

func gitShort(repo string) string {
	gitShortOnce.Do(func() {
		out, err := exec.Command("git", "-C", repo, "rev-parse", "--short", "HEAD").Output()
		if err != nil {
			gitShortVal = "unknown"
			return
		}
		gitShortVal = strings.TrimSpace(string(out))
	})
	return gitShortVal
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
	if runtime.GOOS != "darwin" {
		return os.Getenv(keychainEnv(name))
	}
	account := os.Getenv("MIRABILIS_KEYCHAIN_ACCOUNT")
	if account == "" {
		if u := os.Getenv("USER"); u != "" {
			account = u
		} else {
			account = "mirabilis"
		}
	}

	out, err := exec.Command("security", "find-generic-password", "-a", account, "-s", "mirabilis-"+name+"-token", "-w").Output()
	if err != nil {
		return os.Getenv(keychainEnv(name))
	}
	return strings.TrimSpace(string(out))
}

func ensureDocker(_ context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("Docker is not installed — run 'make bootstrap'")
	}
	if _, err := exec.LookPath("devcontainer"); err != nil {
		return fmt.Errorf("devcontainer CLI is missing — run 'make bootstrap'")
	}
	if dockerReachable() {
		return nil
	}
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("Docker daemon is not running")
	}
	_ = exec.Command("open", "-a", "Docker").Run()
	for i := 0; i < 60; i++ {
		if dockerReachable() {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("Docker did not come up — open Docker Desktop and run mirabilis again")
}

func dockerReachable() bool { return exec.Command("docker", "info").Run() == nil }
