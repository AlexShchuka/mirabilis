#!/usr/bin/env bash
set -euo pipefail
url="${1:-}"
[ -n "$url" ] || exit 0
case "$url" in
  http://*|https://*) ;;
  *) exit 0 ;;
esac
printf '%s\n' "$url" | socat - TCP:host.docker.internal:"${MIRABILIS_OPENER_PORT:-38129}" 2>/dev/null || true
