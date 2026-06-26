package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

const (
	updateTimeout     = 10 * time.Minute
	selfUpdateTimeout = 10 * time.Minute
	selfUpdateNode    = "selfupdate"
)

func UpdateEcosystem(ctx context.Context, d Deps) error {
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	argv := containerArgv("mirabilis", "provision", "--phase", "update")
	if _, err := exec.Run(ctx, d.Runner, exec.Spec{Argv: argv}); err != nil {
		return fmt.Errorf("steps: update ecosystem: %w", err)
	}
	return nil
}

func SelfUpdate(ctx context.Context, d Deps, repo string) error {
	ctx, cancel := context.WithTimeout(ctx, selfUpdateTimeout)
	defer cancel()
	cmds := [][]string{
		{"git", "-C", repo, "pull", "--ff-only"},
		{"make", "-C", repo, "install"},
	}
	for _, argv := range cmds {
		if err := streamSelfUpdate(ctx, d, argv); err != nil {
			d.Obs.Set(selfUpdateNode, obs.StateDegraded, err.Error())
			return fmt.Errorf("steps: self-update: %w", err)
		}
	}
	d.Obs.Set(selfUpdateNode, obs.StateOK, "")
	return nil
}

func streamSelfUpdate(ctx context.Context, d Deps, argv []string) error {
	log := d.Obs.Logger(selfUpdateNode)
	var exitErr error
	var tail []string
	for ev := range d.Runner.Stream(ctx, exec.Spec{Argv: argv}) {
		switch ev.Kind {
		case exec.KindStarted:
			log.Info("started", "argv", ev.Argv)
		case exec.KindStdout, exec.KindStderr:
			if line := ev.Line; line != "" {
				log.Info("line", "line", line)
				tail = appendTail(tail, line)
			}
		case exec.KindExited:
			exitErr = ev.Err
		}
	}
	if exitErr != nil && len(tail) > 0 {
		return fmt.Errorf("%w: %s", exitErr, tail[len(tail)-1])
	}
	return exitErr
}

func appendTail(tail []string, line string) []string {
	tail = append(tail, line)
	if len(tail) > streamTailLines {
		tail = tail[len(tail)-streamTailLines:]
	}
	return tail
}
