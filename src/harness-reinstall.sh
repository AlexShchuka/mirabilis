#!/usr/bin/env bash
set -uo pipefail
export HOME=/home/node

log() { printf '[harness-reinstall] %s\n' "$*" >&2; }

command -v claude >/dev/null 2>&1 || {
  log "claude not on PATH; nothing to do"
  exit 0
}

claude plugin marketplace add AlexShchuka/neuro-matrix >/dev/null 2>&1 ||
  claude plugin marketplace update neuro-matrix >/dev/null 2>&1 || true
claude plugin install neuro-matrix@neuro-matrix --scope user 2>&1 || true
claude plugin update neuro-matrix >/dev/null 2>&1 || true
claude plugin list 2>/dev/null | grep -q neuro-matrix ||
  {
    log "WARN: neuro-matrix not installed after reinstall — check git/network"
    exit 1
  }

NM_DIR="$(printf '%s\n' "$HOME"/.claude/plugins/cache/*/neuro-matrix/*/ | sort -V | tail -n1)"
[ -d "$NM_DIR" ] || NM_DIR=""
[ -n "$NM_DIR" ] && ln -sfn "${NM_DIR%/}" "$HOME/.neuro-matrix"

log "neuro-matrix reinstalled"
