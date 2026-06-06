#!/usr/bin/env bash
set -euo pipefail
input="$(cat)"
tool="$(printf '%s' "$input" | jq -r '.tool_name // ""' 2>/dev/null || true)"
case "$tool" in
  Write|Edit|MultiEdit|NotebookEdit) ;;
  *) exit 0 ;;
esac
path="$(printf '%s' "$input" | jq -r '.tool_input.file_path // .tool_input.notebook_path // ""' 2>/dev/null || true)"
[ -n "$path" ] || exit 0
case "$path" in
  "$HOME"/.neuro-matrix|"$HOME"/.neuro-matrix/*|\
  "$HOME"/.claude/plugins|"$HOME"/.claude/plugins/*|\
  "$HOME"/.claude/settings.json|\
  "$HOME"/.claude/.credentials.json|\
  "$HOME"/.config/gh|"$HOME"/.config/gh/*|\
  /etc/*|/usr/*|/opt/mirabilis|/opt/mirabilis/*) ;;
  *) exit 0 ;;
esac
marker="${TMPDIR:-/tmp}/mirabilis-protect-approved"
if [ -f "$marker" ]; then
  now="$(date +%s)"
  mt="$(stat -c %Y "$marker" 2>/dev/null || echo "$now")"
  rm -f "$marker"
  [ $(( now - mt )) -le 300 ] && exit 0
fi
printf 'protect-critical: "%s" is a protected path (harness / credentials / system).\nRequire EXPLICIT user approval before editing it. On approval: touch %s and retry.\n' "$path" "$marker" >&2
exit 2
