package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

const updateTimeout = 10 * time.Minute

// UpdateEcosystem re-hydrates the four ecosystem repos on demand by running the
// container-side update provision phase, which reuses the ecosystem clone-or-pull step.
// It is the menu UPDATE action's worker: one command that refreshes the repos without a
// full launch pass.
func UpdateEcosystem(ctx context.Context, d Deps) error {
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	argv := containerArgv("mirabilis", "provision", "--phase", "update")
	if _, err := exec.Run(ctx, d.Runner, exec.Spec{Argv: argv}); err != nil {
		return fmt.Errorf("steps: update ecosystem: %w", err)
	}
	return nil
}
