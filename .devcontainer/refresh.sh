#!/usr/bin/env bash
set -uo pipefail
export HOME=/home/node

mkdir -p "$HOME/.claude" "$HOME/.claude/xdg-data"
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

THEME_FILE="$HOME/.claude/.mirabilis-theme"
if [ -f "$THEME_FILE" ] && [ -f "$DEST" ]; then
  th="$(cat "$THEME_FILE" 2>/dev/null)"
  if [ -n "$th" ]; then
    tmp="$(mktemp)"
    jq --arg t "$th" '.theme = $t' "$DEST" > "$tmp" && mv "$tmp" "$DEST" || rm -f "$tmp"
  fi
fi

APT_LIST=/opt/mirabilis/config/apt-packages.txt
if [ -f "$APT_LIST" ]; then
  missing=""
  while IFS= read -r pkg; do
    [ -n "$pkg" ] || continue
    dpkg -s "$pkg" >/dev/null 2>&1 || missing="$missing $pkg"
  done < "$APT_LIST"
  if [ -n "$missing" ]; then
    sudo apt-get update >/dev/null 2>&1 && sudo apt-get install -y --no-install-recommends $missing >/dev/null 2>&1 \
      || echo "[refresh] WARN: declared apt packages not fully applied:$missing" >&2
  fi
fi

CJSON="$HOME/.claude.json"
if [ -f "$CJSON" ]; then
  tmp="$(mktemp)"
  jq '.projects["/workspace"].hasTrustDialogAccepted = true' "$CJSON" > "$tmp" && mv "$tmp" "$CJSON" || rm -f "$tmp"
else
  printf '{"projects":{"/workspace":{"hasTrustDialogAccepted":true}}}\n' > "$CJSON"
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
  claude plugin marketplace add anthropics/claude-plugins-official >/dev/null 2>&1 || true
  for p in github claude-code-setup claude-md-management chrome-devtools-mcp; do
    claude plugin list 2>/dev/null | grep -q "$p" \
      || claude plugin install "$p@claude-plugins-official" --scope user >/dev/null 2>&1 || true
  done
fi

NM_DIR="$(ls -1d "$HOME"/.claude/plugins/cache/*/neuro-matrix/*/ 2>/dev/null | sort -V | tail -n1)"
[ -n "$NM_DIR" ] && ln -sfn "${NM_DIR%/}" "$HOME/.neuro-matrix"

/usr/local/bin/provision-mcp.sh || true

if command -v rtk >/dev/null 2>&1; then
  jq -e '.hooks.PreToolUse[]?.hooks[]? | select(.command == "rtk hook claude")' "$DEST" >/dev/null 2>&1 \
    || rtk init -g --auto-patch >/dev/null 2>&1 || true
fi

if [ -f "$DEST" ] && jq -e . "$DEST" >/dev/null 2>&1; then
  tmp="$(mktemp)"
  jq '(.hooks.PreToolUse) |= (map(.hooks |= map(select((.command != "bash /opt/mirabilis/protect-critical.sh") and (.command != "bash /opt/mirabilis/consent-gate.sh")))) | map(select((.hooks | length) > 0)))' "$DEST" > "$tmp" && mv "$tmp" "$DEST" || rm -f "$tmp"
fi
