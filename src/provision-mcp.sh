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

reg_stdio() {
  local name="$1"
  shift
  claude mcp remove "$name" --scope user >/dev/null 2>&1 || true
  claude mcp add --scope user --transport stdio "$name" -- "$@" \
    >/dev/null 2>&1 && log "registered $name (stdio)" || log "WARN: failed to register $name"
}

if [[ -n "${CONTEXT7_API_KEY:-}" ]]; then
  reg context7 http "https://mcp.context7.com/mcp" "CONTEXT7_API_KEY: ${CONTEXT7_API_KEY}"
else
  reg context7 http "https://mcp.context7.com/mcp"
fi

reg_stdio sequential-thinking npx -y @modelcontextprotocol/server-sequential-thinking

if command -v uvx >/dev/null 2>&1; then
  reg_stdio arxiv-mcp-server uvx arxiv-mcp-server
  reg_stdio docling uvx --from "docling-mcp[local]" docling-mcp-server --transport stdio
else
  log "WARN: uvx not on PATH; skipping arxiv-mcp-server and docling MCP servers"
fi

claude mcp list >/dev/null 2>&1 || true
