#!/bin/bash
set -uo pipefail

: "${SPIKE_SESSION_KEY:?}" "${SPIKE_UPSTREAM:?}"

export ANTHROPIC_TARGET_API_URL="$SPIKE_UPSTREAM"
export HEADROOM_STATELESS=true
/opt/headroom/bin/headroom proxy --host 127.0.0.1 --port 8787 --no-telemetry &

ready=0
for _ in $(seq 1 60); do
  if curl -fsS http://127.0.0.1:8787/stats >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
if [ "$ready" != 1 ]; then
  echo "SPIKE: headroom did not come up"
  exit 3
fi
echo "SPIKE: headroom up, upstream=$SPIKE_UPSTREAM"

export ANTHROPIC_BASE_URL=http://127.0.0.1:8787
export ANTHROPIC_AUTH_TOKEN="$SPIKE_SESSION_KEY"

echo "SPIKE: === text reply ==="
claude -p "Reply with exactly: pong"
rc1=$?
echo "SPIKE: text rc=$rc1"

echo "SPIKE: === streaming ==="
claude -p "Count from 1 to 3, one number per line." --output-format stream-json --verbose > /tmp/stream.json
rc2=$?
head -c 3000 /tmp/stream.json
echo ""
echo "SPIKE: stream rc=$rc2"

if [ "$rc1" = 0 ] && [ "$rc2" = 0 ]; then
  echo "SPIKE: RESULT=PASS"
  exit 0
fi
echo "SPIKE: RESULT=FAIL rc1=$rc1 rc2=$rc2"
exit 1
