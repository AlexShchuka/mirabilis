#!/usr/bin/env bash
set -uo pipefail
command -v gh  >/dev/null 2>&1 || exit 0
command -v git >/dev/null 2>&1 || exit 0
user="$(gh api user 2>/dev/null || true)"
[ -n "$user" ] || exit 0
login="$(printf '%s' "$user" | jq -r '.login // empty')"
[ -n "$login" ] || exit 0
name="$(printf '%s' "$user" | jq -r '.name // empty')"; [ -n "$name" ] || name="$login"
email="$(printf '%s' "$user" | jq -r '.email // empty')"
[ -n "$email" ] || email="$(printf '%s' "$user" | jq -r '.id')+$login@users.noreply.github.com"
git config --global user.name  "$name"  2>/dev/null || true
git config --global user.email "$email" 2>/dev/null || true
