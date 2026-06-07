#!/usr/bin/env bash
set -uo pipefail
export HOME=/home/node

log() { printf '[entrypoint] %s\n' "$*" >&2; }

if [ -x /opt/mirabilis/refresh.sh ]; then
  /opt/mirabilis/refresh.sh || log "WARN: per-start setup returned non-zero — host preflight will report and gate"
else
  log "WARN: per-start setup script missing at /opt/mirabilis/refresh.sh"
fi

if command -v claude >/dev/null 2>&1; then
  claude plugin list 2>/dev/null | grep -q neuro-matrix \
    || log "WARN: neuro-matrix harness not confirmed — host preflight will gate before launching the agent"
fi

exec sleep infinity
