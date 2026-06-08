#!/usr/bin/env bash

ensure_tools() {
  if ! command -v docker >/dev/null 2>&1 || ! command -v devcontainer >/dev/null 2>&1; then
    echo "mirabilis: installing prerequisites (make bootstrap)…" >&2
    make -C "$REPO" bootstrap || die "bootstrap failed — see the output above"
  fi
  command -v mirabilis >/dev/null 2>&1 || {
    echo "mirabilis: putting the 'mirabilis' command on your PATH (make install)…" >&2
    make -C "$REPO" install || true
  }
}

ensure_docker() {
  command -v docker >/dev/null 2>&1 || die "Docker is not installed — run 'make bootstrap'."
  command -v devcontainer >/dev/null 2>&1 || die "devcontainer CLI is missing — run 'make bootstrap'."
  docker info >/dev/null 2>&1 && return 0
  [ -d /Applications/Docker.app ] || die "Docker daemon is not running — start Docker Desktop."
  echo "mirabilis: starting Docker Desktop…" >&2
  open -a Docker
  for _ in {1..60}; do docker info >/dev/null 2>&1 && return 0; sleep 2; done
  die "Docker did not come up — open Docker Desktop and run 'mirabilis' again."
}

dc_up() {
  local rc=0
  if [ -t 2 ]; then
    : > "$BUILD_LOG"
    printf 'mirabilis: building the workspace… (full log: %s)\n' "$BUILD_LOG" >&2
    "$DC" up --workspace-folder "$REPO" "$@" >"$BUILD_LOG" 2>&1 </dev/null &
    local pid=$! n=0 cols line
    local frames=(⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏)
    cols="$(tput cols 2>/dev/null || echo 80)"
    while kill -0 "$pid" 2>/dev/null; do
      line="$(tail -n1 "$BUILD_LOG" 2>/dev/null | tr -d '\r' | sed -E 's/^\[[0-9T:.Z+-]+\] *//; s/^#[0-9]+ [0-9.]+ +//')"
      printf '\r\033[2K\033[36m%s\033[0m \033[2m%.*s\033[0m' \
        "${frames[n % 10]}" "$(( cols > 12 ? cols - 4 : 8 ))" "${line:-starting…}" >&2
      n=$((n + 1))
      sleep 0.2
    done
    wait "$pid" || rc=$?
    printf '\r\033[2K' >&2
  else
    echo "mirabilis: building / starting the workspace (log: $BUILD_LOG)…" >&2
    "$DC" up --workspace-folder "$REPO" "$@" >"$BUILD_LOG" 2>&1 || rc=$?
  fi
  if [ "$rc" -ne 0 ]; then
    echo "mirabilis: workspace failed to start — last 40 log lines:" >&2
    tail -n 40 "$BUILD_LOG" >&2
    die "full build log: $BUILD_LOG"
  fi
}
dx()      { "$DC" exec --workspace-folder "$REPO" "$@"; }
dxq()     { "$DC" exec --workspace-folder "$REPO" "$@" </dev/null 2>/dev/null; }

rebuild_image() {
  stop_proxy
  ( cd "$REPO" && . "$REPO/src/env.sh" && docker compose -p mirabilis -f docker-compose.yml down ) >/dev/null 2>&1 || true
  docker image rm mirabilis:local >/dev/null 2>&1 || true
  ensure_proxy
}

prepare_container() {
  ensure_proxy
  if container_running && ! is_stale; then
    return 0
  fi
  if container_exists && is_stale; then
    echo "mirabilis: the workspace (${1:-old}) is behind your checkout ($(repo_version)) — rebuilding (memory, auth and /workspace are kept)." >&2
    rebuild_image
  fi
  dc_up
  ensure_github
  ensure_claude
}
