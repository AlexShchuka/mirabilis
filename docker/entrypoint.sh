#!/usr/bin/env bash
set -uo pipefail

log() { printf '[entrypoint] %s\n' "$*" >&2; }

CONFIG_SEED=/opt/mirabilis/config/settings.json
MARKETPLACE_DIR=/opt/mirabilis/marketplace
MARKETPLACE_NAME=mirabilis
PLUGIN_NAME=neuro-matrix

if [[ "$(id -u)" -eq 0 ]]; then
  /usr/local/bin/init-firewall.sh || log "WARN: firewall init failed"
  chown coder:coder /home/coder 2>/dev/null || true
  exec gosu coder "$0" "$@"
fi

export HOME=/home/coder

mkdir -p "$HOME/.claude"
if [[ ! -f "$HOME/.claude/settings.json" && -f "$CONFIG_SEED" ]]; then
  cp "$CONFIG_SEED" "$HOME/.claude/settings.json"
  log "seeded ~/.claude/settings.json"
fi

if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  export GH_TOKEN="${GH_TOKEN:-$GITHUB_TOKEN}"
  gh auth setup-git 2>/dev/null && log "git credential helper -> gh" || log "WARN: gh auth setup-git failed"
else
  log "WARN: GITHUB_TOKEN unset — private clone & GitHub MCP disabled"
fi

if command -v claude >/dev/null 2>&1 && [[ -f "$MARKETPLACE_DIR/.claude-plugin/marketplace.json" ]]; then
  claude plugin marketplace add "$MARKETPLACE_DIR" >/dev/null 2>&1 \
    && log "marketplace '$MARKETPLACE_NAME' registered" \
    || claude plugin marketplace update "$MARKETPLACE_NAME" >/dev/null 2>&1 || true
  claude plugin install "${PLUGIN_NAME}@${MARKETPLACE_NAME}" --scope user >/dev/null 2>&1 \
    && log "plugin ${PLUGIN_NAME} installed" \
    || { claude plugin update "$PLUGIN_NAME" >/dev/null 2>&1 \
         && log "plugin ${PLUGIN_NAME} updated" \
         || log "WARN: could not preinstall ${PLUGIN_NAME}"; }
fi

/usr/local/bin/provision-mcp.sh || log "WARN: MCP provisioning reported errors"

log "ready — run the agent with: make claude"
exec "$@"
