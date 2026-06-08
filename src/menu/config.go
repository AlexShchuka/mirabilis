package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	huh "charm.land/huh/v2"
)

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func doPlugins(ctx context.Context, r Runner) error {
	raw, _ := r.Container(ctx, "bash", "-lc", `sed -e '/^#/d' -e '/^[[:space:]]*$/d' /opt/mirabilis/config/plugins.txt 2>/dev/null`)
	catalog := splitLines(raw)
	if len(catalog) == 0 {
		return errors.New("plugins: no catalog found")
	}
	disabledRaw, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-plugins-disabled" 2>/dev/null`)
	disabled := splitLines(disabledRaw)

	opts := make([]huh.Option[string], 0, len(catalog))
	for _, p := range catalog {
		opts = append(opts, huh.NewOption(p, p).Selected(!contains(disabled, p)))
	}
	var chosen []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Плагины (пробел — переключить, Enter — ок)").
			Options(opts...).Value(&chosen)))
	ok, err := runForm(form)
	if err != nil && !errors.Is(err, huh.ErrUserAborted) {
		return err
	}
	if !ok {
		return nil
	}
	var newDisabled []string
	for _, p := range catalog {
		if !contains(chosen, p) {
			newDisabled = append(newDisabled, p)
		}
	}
	_, err = r.Container(ctx, "env", "MDIS="+strings.Join(newDisabled, "\n"),
		"bash", "-lc", `printf '%s' "$MDIS" > "$HOME/.claude/.mirabilis-plugins-disabled"`)
	return err
}

func doHarness(ctx context.Context, r Runner) error {
	cur := "on"
	if pref, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`); strings.TrimSpace(pref) == "skip" {
		cur = "off"
	}
	choice := cur
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("neuro-matrix харнес").
			Options(
				huh.NewOption("Включить", "on"),
				huh.NewOption("Выключить", "off"),
				huh.NewOption("Переустановить", "reinstall"),
			).Value(&choice)))
	ok, err := runForm(form)
	if err != nil && !errors.Is(err, huh.ErrUserAborted) {
		return err
	}
	if !ok {
		return nil
	}
	switch choice {
	case "off":
		_, err = r.Container(ctx, "bash", "-lc", `echo skip > "$HOME/.claude/.mirabilis-harness"`)
	case "on":
		_, err = r.Container(ctx, "bash", "-lc", `echo install > "$HOME/.claude/.mirabilis-harness"`)
	case "reinstall":
		_, _ = r.Container(ctx, "bash", "-lc", `echo install > "$HOME/.claude/.mirabilis-harness"`)
		_, err = r.Container(ctx, "bash", "/usr/local/bin/harness-reinstall.sh")
	}
	return err
}

func doVSCode(ctx context.Context, r Runner) error {
	code, err := resolveCode()
	if err != nil {
		return err
	}
	if !containerRunning(ctx, r) {
		up := exec.CommandContext(ctx, "devcontainer", "up", "--workspace-folder", r.Repo())
		up.Env = composeEnv(r.Repo())
		if e := up.Run(); e != nil {
			return e
		}
	}
	enc := hex.EncodeToString([]byte(`{"containerName":"/mirabilis"}`))
	uri := "vscode-remote://attached-container+" + enc + "/workspace"
	return exec.Command(code, "--folder-uri", uri).Run()
}

func resolveCode() (string, error) {
	if p, err := exec.LookPath("code"); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	for _, b := range []string{
		"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
		filepath.Join(home, "Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"),
		"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code",
	} {
		if fi, err := os.Stat(b); err == nil && !fi.IsDir() {
			return b, nil
		}
	}
	return "", fmt.Errorf("VS Code not found — install it from https://code.visualstudio.com")
}
