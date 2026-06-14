package sandbox

import (
	"context"
	"errors"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

var ErrNoClipboard = errors.New("no clipboard utility found (xclip, xsel, wl-copy, clip.exe)")

type clipCandidate struct {
	name string
	argv func(path string) []string
}

var clipCandidates = []clipCandidate{
	{"xclip", func(p string) []string { return []string{p, "-selection", "clipboard"} }},
	{"xsel", func(p string) []string { return []string{p, "--clipboard", "--input"} }},
	{"wl-copy", func(p string) []string { return []string{p} }},
	{"clip.exe", func(p string) []string { return []string{p} }},
}

func CopyText(ctx context.Context, runner exec.Runner, text string) error {
	for _, c := range clipCandidates {
		if p, err := lookPath(c.name); err == nil {
			_, err = exec.Run(ctx, runner, exec.Spec{
				Argv:  c.argv(p),
				Stdin: strings.NewReader(text),
			})
			return err
		}
	}
	return ErrNoClipboard
}
