package provision

import (
	"context"
	"strconv"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
)

type Status struct {
	Harness       string
	CommitsBehind int
	Stale         bool
	ContainerUp   bool
	ProvisionWarn string
}

func ComputeStatus(ctx context.Context, r runner.Runner) Status {
	var st Status
	if out, err := r.Host(ctx, "git", "-C", r.Repo(), "rev-list", "--count", "HEAD..origin/main"); err == nil {
		st.CommitsBehind, _ = strconv.Atoi(strings.TrimSpace(out))
	}
	if runtime.ContainerExists(ctx, r) && runtime.IsStale(ctx, r) {
		st.Stale = true
	}
	st.ContainerUp = runtime.ContainerRunning(ctx, r)
	st.Harness = harnessStatus(ctx, r)
	st.ProvisionWarn = readProvisionStatus(ctx, r)
	return st
}

func readProvisionStatus(ctx context.Context, r runner.Runner) string {
	out, err := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-provision-status" 2>/dev/null`)
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(out)
	if v == "ok" || v == "" {
		return ""
	}
	return v
}

func harnessStatus(ctx context.Context, r runner.Runner) string {
	pref, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`)
	if strings.TrimSpace(pref) == "skip" {
		return "off"
	}
	if !runtime.ContainerRunning(ctx, r) {
		return "unknown"
	}
	if _, err := r.Container(ctx, "bash", "-lc", "claude plugin list 2>/dev/null | grep -q neuro-matrix"); err != nil {
		return "missing"
	}
	return "on"
}
