package main

import (
	"context"
	"strconv"
	"strings"
)

func computeStatus(ctx context.Context, r Runner) Status {
	var st Status
	if out, err := r.Host(ctx, "git", "-C", r.Repo(), "rev-list", "--count", "HEAD..origin/main"); err == nil {
		st.CommitsBehind, _ = strconv.Atoi(strings.TrimSpace(out))
	}
	if containerExists(ctx, r) && isStale(ctx, r) {
		st.Stale = true
	}
	st.Harness = harnessStatus(ctx, r)
	return st
}

func containerExists(ctx context.Context, r Runner) bool {
	_, err := r.Host(ctx, "docker", "container", "inspect", "mirabilis")
	return err == nil
}

func harnessStatus(ctx context.Context, r Runner) string {
	if pref, _ := r.Container(ctx, "bash", "-lc", `cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null`); strings.TrimSpace(pref) == "skip" {
		return "off"
	}
	if !containerRunning(ctx, r) {
		return "unknown"
	}
	if _, err := r.Container(ctx, "bash", "-lc", `claude plugin list 2>/dev/null | grep -q neuro-matrix`); err != nil {
		return "missing"
	}
	return "on"
}
