#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
REPO="$PWD"
. "$REPO/src/env.sh"

GITHUB_TOKEN="$(./src/token.sh get gh 2>/dev/null || true)"
CLAUDE_CODE_OAUTH_TOKEN="$(./src/token.sh get claude 2>/dev/null || true)"
CONTEXT7_API_KEY="$(./src/token.sh get context7 2>/dev/null || true)"
TELEGRAM_BOT_TOKEN="$(./src/token.sh get telegram-token 2>/dev/null || true)"
TELEGRAM_CHAT_ID="$(./src/token.sh get telegram-chat 2>/dev/null || true)"
export GITHUB_TOKEN
export GH_TOKEN="$GITHUB_TOKEN"
export CLAUDE_CODE_OAUTH_TOKEN
export CONTEXT7_API_KEY
export TELEGRAM_BOT_TOKEN
export TELEGRAM_CHAT_ID

exec devcontainer "$@"
