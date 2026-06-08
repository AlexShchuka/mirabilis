#!/usr/bin/env bash
set -uo pipefail
export HOME=/home/node

log() { printf '[harness-reinstall] %s\n' "$*" >&2; }

command -v claude >/dev/null 2>&1 || { log "claude not on PATH; nothing to do"; exit 0; }

if [ ! -f /opt/mirabilis/marketplace/.claude-plugin/marketplace.json ]; then
  log "marketplace manifest missing at /opt/mirabilis/marketplace; cannot reinstall"
  exit 1
fi

claude plugin marketplace add /opt/mirabilis/marketplace >/dev/null 2>&1 \
  || claude plugin marketplace update mirabilis >/dev/null 2>&1 || true
claude plugin install neuro-matrix@mirabilis --scope user 2>&1 || true
claude plugin update neuro-matrix >/dev/null 2>&1 || true
claude plugin list 2>/dev/null | grep -q neuro-matrix \
  || { log "WARN: neuro-matrix not installed after reinstall — check git/network"; exit 1; }

NM_DIR="$(ls -1d "$HOME"/.claude/plugins/cache/*/neuro-matrix/*/ 2>/dev/null | sort -V | tail -n1)"
[ -n "$NM_DIR" ] && ln -sfn "${NM_DIR%/}" "$HOME/.neuro-matrix"

log "neuro-matrix reinstalled"
