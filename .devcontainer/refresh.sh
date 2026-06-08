#!/usr/bin/env bash
set -uo pipefail
export HOME=/home/node

mkdir -p "$HOME/.claude" "$HOME/.claude/xdg-data"
SEED=/opt/mirabilis/config/settings.json
DEST="$HOME/.claude/settings.json"
if [ -f "$SEED" ]; then
  if [ -f "$DEST" ]; then
    tmp="$(mktemp)"
    if jq -s '.[0] * .[1] | del(.sandbox)' "$DEST" "$SEED" >"$tmp"; then
      mv "$tmp" "$DEST"
    else
      cp "$SEED" "$DEST"
    fi
  else
    cp "$SEED" "$DEST"
  fi
fi

THEME_FILE="$HOME/.claude/.mirabilis-theme"
if [ -f "$THEME_FILE" ] && [ -f "$DEST" ]; then
  th="$(cat "$THEME_FILE" 2>/dev/null)"
  if [ -n "$th" ]; then
    tmp="$(mktemp)"
    if jq --arg t "$th" '.theme = $t' "$DEST" >"$tmp"; then
      mv "$tmp" "$DEST"
    else
      rm -f "$tmp"
    fi
  fi
fi

APT_LIST=/opt/mirabilis/config/apt-packages.txt
if [ -f "$APT_LIST" ]; then
  missing=()
  while IFS= read -r pkg; do
    [ -n "$pkg" ] || continue
    dpkg -s "$pkg" >/dev/null 2>&1 || missing+=("$pkg")
  done <"$APT_LIST"
  if [ "${#missing[@]}" -gt 0 ]; then
    sudo apt-get update >/dev/null 2>&1 && sudo apt-get install -y --no-install-recommends "${missing[@]}" >/dev/null 2>&1 ||
      echo "[refresh] WARN: declared apt packages not fully applied: ${missing[*]}" >&2
  fi
fi

CJSON="$HOME/.claude.json"
if [ -f "$CJSON" ]; then
  tmp="$(mktemp)"
  if jq '.projects["/workspace"].hasTrustDialogAccepted = true' "$CJSON" >"$tmp"; then
    mv "$tmp" "$CJSON"
  else
    rm -f "$tmp"
  fi
else
  printf '{"projects":{"/workspace":{"hasTrustDialogAccepted":true}}}\n' >"$CJSON"
fi

RULES_SRC=/opt/mirabilis/config/memory/rules
RULES_DST="$HOME/.claude/rules"
if [ -d "$RULES_SRC" ]; then
  mkdir -p "$RULES_DST"
  for f in "$RULES_SRC"/*.md; do
    [ -e "$f" ] || continue
    dst="$RULES_DST/$(basename "$f")"
    [ -e "$dst" ] || cp "$f" "$dst"
  done
fi

GITHUB_TOKEN="${GITHUB_TOKEN:-$(gh auth token 2>/dev/null || true)}"
if [ -n "$GITHUB_TOKEN" ]; then
  export GITHUB_TOKEN GH_TOKEN="$GITHUB_TOKEN"
  gh auth setup-git 2>/dev/null || true
  bash /usr/local/bin/git-identity.sh || true
fi

HARNESS_CHOICE="$(cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null || echo install)"
PLUGINS_CATALOG=/opt/mirabilis/config/plugins.txt
PLUGINS_DISABLED="$HOME/.claude/.mirabilis-plugins-disabled"

if command -v claude >/dev/null 2>&1; then
  if [ "$HARNESS_CHOICE" != skip ] && [ -f /opt/mirabilis/marketplace/.claude-plugin/marketplace.json ]; then
    claude plugin marketplace add /opt/mirabilis/marketplace >/dev/null 2>&1 ||
      claude plugin marketplace update mirabilis >/dev/null 2>&1 || true
    claude plugin install neuro-matrix@mirabilis --scope user 2>&1 || true
    claude plugin update neuro-matrix >/dev/null 2>&1 || true
    claude plugin list 2>/dev/null | grep -q neuro-matrix ||
      echo "[refresh] WARN: neuro-matrix selected but not installed — check git/network" >&2
  fi
  if [ -f "$PLUGINS_CATALOG" ]; then
    claude plugin marketplace add anthropics/claude-plugins-official >/dev/null 2>&1 || true
    while IFS= read -r p; do
      [ -n "$p" ] || continue
      case "$p" in \#*) continue ;; esac
      grep -qxF "$p" "$PLUGINS_DISABLED" 2>/dev/null && continue
      claude plugin list 2>/dev/null | grep -q "${p%@*}" ||
        claude plugin install "$p" --scope user >/dev/null 2>&1 || true
    done <"$PLUGINS_CATALOG"
  fi
fi

if command -v jq >/dev/null 2>&1 && [ -f "$DEST" ]; then
  enabled="$(
    [ "$HARNESS_CHOICE" != skip ] && printf 'neuro-matrix@mirabilis\n'
    if [ -f "$PLUGINS_CATALOG" ]; then
      while IFS= read -r p; do
        [ -n "$p" ] || continue
        case "$p" in \#*) continue ;; esac
        grep -qxF "$p" "$PLUGINS_DISABLED" 2>/dev/null && continue
        printf '%s\n' "$p"
      done <"$PLUGINS_CATALOG"
    fi
  )"
  obj="$(printf '%s\n' "$enabled" | jq -R . | jq -s 'map(select(length>0)) | reduce .[] as $p ({}; .[$p]=true)')"
  tmp="$(mktemp)"
  if jq --argjson e "$obj" '.enabledPlugins = $e' "$DEST" >"$tmp"; then
    mv "$tmp" "$DEST"
  else
    rm -f "$tmp"
  fi
fi

NM_DIR="$(printf '%s\n' "$HOME"/.claude/plugins/cache/*/neuro-matrix/*/ | sort -V | tail -n1)"
[ -d "$NM_DIR" ] || NM_DIR=""
[ -n "$NM_DIR" ] && ln -sfn "${NM_DIR%/}" "$HOME/.neuro-matrix"

SKILLS_DIR="$HOME/.claude/skills"
IC_DIR="$SKILLS_DIR/interview-coach"
if command -v git >/dev/null 2>&1; then
  mkdir -p "$SKILLS_DIR"
  if [ -d "$IC_DIR/.git" ]; then
    git -C "$IC_DIR" pull --ff-only >/dev/null 2>&1 || true
  elif [ ! -e "$IC_DIR" ]; then
    git clone --depth 1 https://github.com/noamseg/interview-coach-skill.git "$IC_DIR" >/dev/null 2>&1 ||
      echo "[refresh] WARN: interview-coach skill not installed — check network" >&2
  fi
fi

/usr/local/bin/provision-mcp.sh || true

if command -v rtk >/dev/null 2>&1; then
  jq -e '.hooks.PreToolUse[]?.hooks[]? | select(.command == "rtk hook claude")' "$DEST" >/dev/null 2>&1 ||
    rtk init -g --auto-patch >/dev/null 2>&1 || true
fi
