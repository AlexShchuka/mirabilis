package sandbox

import (
	"context"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

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
	return nil
}
