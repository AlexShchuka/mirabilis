#!/usr/bin/env bash

repo_version() { git -C "$REPO" rev-parse --short HEAD 2>/dev/null || echo unknown; }
container_version() { docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' mirabilis 2>/dev/null | sed -n 's/^MIRABILIS_VERSION=//p' | tr -d '[:space:]'; }
container_stacks() { docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' mirabilis 2>/dev/null | sed -n 's/^MIRABILIS_STACKS=//p' | tr -d '[:space:]'; }
container_exists() { docker container inspect mirabilis >/dev/null 2>&1; }
container_running() { [ "$(docker container inspect -f '{{.State.Running}}' mirabilis 2>/dev/null)" = "true" ]; }

is_stale() {
  local src cont
  src="$(repo_version)"
  cont="$(container_version)"
  [ -z "$cont" ] && return 0
  [ "$(container_stacks)" != "$(stacks_current)" ] && return 0
  [ "$src" = unknown ] && return 1
  [ "$cont" != "$src" ]
}

repo_behind() {
  git -C "$REPO" rev-parse --git-dir >/dev/null 2>&1 || {
    echo 0
    return
  }
  GIT_TERMINAL_PROMPT=0 git -C "$REPO" fetch -q origin 2>/dev/null || {
    echo 0
    return
  }
  git -C "$REPO" rev-parse --verify -q origin/main >/dev/null 2>&1 || {
    echo 0
    return
  }
  git -C "$REPO" rev-list --count HEAD..origin/main 2>/dev/null || echo 0
}

nm_status() {
  container_running || {
    echo unknown
    return
  }
  dxq bash -lc 'claude plugin list 2>/dev/null | grep -q neuro-matrix' && echo ok || echo missing
}

pull_latest() {
  if [ -n "$(git -C "$REPO" status --porcelain 2>/dev/null)" ]; then
    echo "mirabilis: the mirabilis repo has local changes — not pulling; commit or stash them first." >&2
    return 1
  fi
  if [ "$(git -C "$REPO" rev-parse --abbrev-ref HEAD 2>/dev/null)" != main ]; then
    git -C "$REPO" checkout main >/dev/null 2>&1 || {
      echo "mirabilis: could not switch to the main branch." >&2
      return 1
    }
  fi
  GIT_TERMINAL_PROMPT=0 git -C "$REPO" pull --ff-only >/dev/null 2>&1 || {
    echo "mirabilis: pull failed (diverged or offline) — resolve manually." >&2
    return 1
  }
}
