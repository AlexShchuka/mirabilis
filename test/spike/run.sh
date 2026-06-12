#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

MODE="${1:-no-inject}"
PORT="${SPIKE_PORT:-8788}"
SESSION_KEY="${SPIKE_SESSION_KEY:-spike-$(openssl rand -hex 16)}"

if ! security find-generic-password -a "$USER" -s mirabilis-claude-token -w >/dev/null 2>&1; then
  echo "SPIKE: no mirabilis-claude-token in keychain — run: claude setup-token, then store it"
  exit 2
fi

docker build -t mirabilis-spike .

INJECT_FLAG=""
if [ "$MODE" = "inject" ]; then INJECT_FLAG="-inject-beta"; fi

go run . -listen "127.0.0.1:$PORT" -session-key "$SESSION_KEY" $INJECT_FLAG &
PROXY_PID=$!
trap 'kill "$PROXY_PID" 2>/dev/null || true' EXIT
sleep 2

set +e
docker run --rm --name mirabilis-spike \
  --add-host host.docker.internal:host-gateway \
  -e SPIKE_SESSION_KEY="$SESSION_KEY" \
  -e SPIKE_UPSTREAM="http://host.docker.internal:$PORT" \
  mirabilis-spike
RC=$?
set -e

echo "SPIKE: container rc=$RC (mode=$MODE)"
exit "$RC"
