#!/usr/bin/env bash
set -uo pipefail

log() { printf '[mcp] %s\n' "$*" >&2; }
command -v claude >/dev/null 2>&1 || { log "claude not on PATH; skipping"; exit 0; }

reg() {
  local name="$1" transport="$2" url="$3" header="${4:-}"
  claude mcp remove "$name" --scope user >/dev/null 2>&1 || true
  if [[ -n "$header" ]]; then
    claude mcp add --scope user --transport "$transport" "$name" "$url" --header "$header" \
      >/dev/null 2>&1 && log "registered $name ($transport)" || log "WARN: failed to register $name"
  else
    claude mcp add --scope user --transport "$transport" "$name" "$url" \
      >/dev/null 2>&1 && log "registered $name ($transport)" || log "WARN: failed to register $name"
  fi
}

if [[ -n "${CONTEXT7_API_KEY:-}" ]]; then
  reg context7 http "https://mcp.context7.com/mcp" "CONTEXT7_API_KEY: ${CONTEXT7_API_KEY}"
else
  reg context7 http "https://mcp.context7.com/mcp"
fi

claude mcp list >/dev/null 2>&1 || true
