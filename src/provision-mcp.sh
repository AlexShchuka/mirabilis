#!/usr/bin/env bash
set -uo pipefail

log() { printf '[mcp] %s\n' "$*" >&2; }
command -v claude >/dev/null 2>&1 || {
  log "claude not on PATH; skipping"
  exit 0
}

reg() {
  local name="$1" transport="$2" url="$3" header="${4:-}"
  claude mcp remove "$name" --scope user >/dev/null 2>&1 || true
  local args=(--scope user --transport "$transport" "$name" "$url")
  [[ -n "$header" ]] && args+=(--header "$header")
  if claude mcp add "${args[@]}" >/dev/null 2>&1; then
    log "registered $name ($transport)"
  else
    log "WARN: failed to register $name"
  fi
}

reg_stdio() {
  local name="$1"
  shift
  claude mcp remove "$name" --scope user >/dev/null 2>&1 || true
  if claude mcp add --scope user --transport stdio "$name" -- "$@" >/dev/null 2>&1; then
    log "registered $name (stdio)"
  else
    log "WARN: failed to register $name"
  fi
}

reg context7 http "https://mcp.context7.com/mcp"

reg_stdio sequential-thinking npx -y @modelcontextprotocol/server-sequential-thinking

if command -v uvx >/dev/null 2>&1; then
  reg_stdio arxiv-mcp-server uvx arxiv-mcp-server
  reg_stdio docling uvx --from "docling-mcp[local]" docling-mcp-server --transport stdio
else
  log "WARN: uvx not on PATH; skipping arxiv-mcp-server and docling MCP servers"
fi

claude mcp list >/dev/null 2>&1 || true
