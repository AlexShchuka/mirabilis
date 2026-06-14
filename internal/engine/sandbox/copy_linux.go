package sandbox

import (
	"context"
	"errors"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

var ErrNoClipboard = errors.New("no clipboard utility found (xclip, xsel, wl-copy)")

func CopyText(ctx context.Context, runner exec.Runner, text string) error {
	for _, name := range []string{"xclip", "xsel", "wl-copy"} {
		if p, err := lookPath(name); err == nil {
			_, err = exec.Run(ctx, runner, exec.Spec{
				Argv:  []string{p},
				Stdin: strings.NewReader(text),
			})
			return err
		}
	}
	return ErrNoClipboard
}
