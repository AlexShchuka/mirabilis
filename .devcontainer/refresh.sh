#!/usr/bin/env bash
set -uo pipefail
export HOME=/home/node

mkdir -p "$HOME/.claude"
SEED=/opt/mirabilis/config/settings.json
DEST="$HOME/.claude/settings.json"
if [ -f "$SEED" ]; then
  if [ -f "$DEST" ]; then
    tmp="$(mktemp)"
    jq -s '.[0] * .[1]' "$DEST" "$SEED" > "$tmp" && mv "$tmp" "$DEST" || cp "$SEED" "$DEST"
  else
    cp "$SEED" "$DEST"
  fi
fi

GITHUB_TOKEN="${GITHUB_TOKEN:-$(gh auth token 2>/dev/null || true)}"
if [ -n "$GITHUB_TOKEN" ]; then
  export GITHUB_TOKEN GH_TOKEN="$GITHUB_TOKEN"
  gh auth setup-git 2>/dev/null || true
fi

if command -v claude >/dev/null 2>&1 && [ -f /opt/mirabilis/marketplace/.claude-plugin/marketplace.json ]; then
  claude plugin marketplace add /opt/mirabilis/marketplace >/dev/null 2>&1 \
    || claude plugin marketplace update mirabilis >/dev/null 2>&1 || true
  claude plugin install neuro-matrix@mirabilis --scope user 2>&1 || true
  claude plugin update neuro-matrix >/dev/null 2>&1 || true
  claude plugin list 2>/dev/null | grep -q neuro-matrix \
    || echo "[refresh] WARN: neuro-matrix not installed — check git/network" >&2
fi

/usr/local/bin/provision-mcp.sh || true
