package sandbox

import (
	"context"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func CopyText(ctx context.Context, runner exec.Runner, text string) error {
	pbcopy, err := lookPath("pbcopy")
	if err != nil {
		return err
	}
	_, err = exec.Run(ctx, runner, exec.Spec{
		Argv:  []string{pbcopy},
		Stdin: strings.NewReader(text),
	})
	return err
}
