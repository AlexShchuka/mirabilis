#!/usr/bin/env bash
set -uo pipefail

token="${TELEGRAM_BOT_TOKEN:-}"
chat="${TELEGRAM_CHAT_ID:-}"
[ -n "$token" ] && [ -n "$chat" ] || exit 0

payload="$(cat 2>/dev/null || true)"

event=""
if command -v jq >/dev/null 2>&1; then
  event="$(printf '%s' "$payload" | jq -r '.hook_event_name // empty' 2>/dev/null || true)"
fi
if [ -z "$event" ]; then
  case "$payload" in
    *'"hook_event_name"'*'"Notification"'*) event="Notification" ;;
    *'"hook_event_name"'*'"Stop"'*)         event="Stop" ;;
  esac
fi

case "$event" in
  Notification) text="❓ mirabilis: нужен твой ответ" ;;
  Stop)         text="✅ mirabilis: задача завершена" ;;
  *)            exit 0 ;;
esac

curl -fsS -m 10 -o /dev/null \
  -X POST "https://api.telegram.org/bot${token}/sendMessage" \
  --data-urlencode "chat_id=${chat}" \
  --data-urlencode "text=${text}" \
  >/dev/null 2>&1 || true

exit 0
