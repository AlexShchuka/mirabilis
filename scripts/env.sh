#!/usr/bin/env bash
: "${REPO:=$PWD}"
set -a
[ -f "$REPO/.env" ] && . "$REPO/.env"
set +a
: "${WORKSPACE_DIR:=$HOME/mirabilis-workspace}"; export WORKSPACE_DIR
: "${MIRABILIS_VERSION:=$(git -C "$REPO" rev-parse --short HEAD 2>/dev/null || echo unknown)}"; export MIRABILIS_VERSION
