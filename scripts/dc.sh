#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
REPO="$PWD"
. "$REPO/scripts/env.sh"
mkdir -p "$WORKSPACE_DIR"

GITHUB_TOKEN="$(./scripts/token.sh get gh 2>/dev/null || true)"
CLAUDE_CODE_OAUTH_TOKEN="$(./scripts/token.sh get claude 2>/dev/null || true)"
CONTEXT7_API_KEY="$(./scripts/token.sh get context7 2>/dev/null || true)"
export GITHUB_TOKEN
export GH_TOKEN="$GITHUB_TOKEN"
export CLAUDE_CODE_OAUTH_TOKEN
export CONTEXT7_API_KEY

exec devcontainer "$@"
