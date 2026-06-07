#!/usr/bin/env bash
set -uo pipefail
set -f

input="$(cat)"
tool="$(printf '%s' "$input" | jq -r '.tool_name // ""' 2>/dev/null || echo "")"

PP_FILE="${MIRABILIS_PROTECTED_PATHS:-/opt/mirabilis/config/protected-paths}"
PROTECTED=()
if [ -f "$PP_FILE" ]; then
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    case "$line" in
      "~/"*) line="$HOME/${line#\~/}" ;;
      "~")   line="$HOME" ;;
    esac
    PROTECTED+=("$line")
  done < "$PP_FILE"
else
  PROTECTED=("$HOME/.neuro-matrix" "$HOME/.claude/plugins" "$HOME/.claude/settings.json" "$HOME/.claude/.credentials.json" "$HOME/.config/gh" /etc /usr /opt/mirabilis)
fi
nm_tgt="$(readlink -f "$HOME/.neuro-matrix" 2>/dev/null || true)"
[ -n "$nm_tgt" ] && PROTECTED+=("$nm_tgt")

is_protected() {
  local path="$1" p
  for p in "${PROTECTED[@]}"; do
    case "$path" in
      "$p"|"$p"/*) return 0 ;;
    esac
  done
  return 1
}

marker="${TMPDIR:-/tmp}/mirabilis-consent-approved"
marker_base="${marker##*/}"

gate_request() {
  local subject="$1" now mt
  if [ -f "$marker" ]; then
    now="$(date +%s)"
    mt="$(stat -c %Y "$marker" 2>/dev/null || stat -f %m "$marker" 2>/dev/null || echo "$now")"
    rm -f "$marker"
    [ $(( now - mt )) -le 300 ] && exit 0
  fi
  {
    printf 'consent-gate: %s\n' "$subject"
    printf 'GATED action: everything outside /workspace and /tmp needs your explicit approval (mirabilis I6).\n'
    printf 'A hook cannot prompt on the terminal (Claude Code v2.1.139+ runs hooks with no controlling terminal),\n'
    printf 'and the agent must not approve on its own — that is a denied gate-bypass.\n'
    printf 'To approve THIS one action, run in the conversation:  ! touch %s\n' "$marker"
    printf 'then ask me to retry. Approval is single-use and expires after 300s.\n'
  } >&2
  exit 2
}

deny() {
  printf 'consent-gate(DENY): %s — never allowed, even with approval.\n' "$1" >&2
  exit 2
}

if [ "$tool" = "Bash" ]; then
  cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // ""' 2>/dev/null || echo "")"

  if printf '%s' "$cmd" | grep -qE '(^|[;&|([:space:]])git([[:space:]]+-[^[:space:]]+)*[[:space:]]+push([[:space:]]|$)'; then
    printf '%s' "$cmd" | grep -qE "(^|[[:space:]:+=/\"'])(main|master)([[:space:]:\"'/]|\$)" && deny "git push to a protected branch (main/master)"
    printf '%s' "$cmd" | grep -qE '([[:space:]](--force([[:space:]]|=|$)|--force-with-lease)|[[:space:]]-[A-Za-z]*f[A-Za-z]*([[:space:]]|$)|[[:space:]]\+[A-Za-z0-9_])' && deny "git force-push"
    printf '%s' "$cmd" | grep -qE '[[:space:]](--mirror|--all)([[:space:]]|$)' && deny "git push --mirror/--all (pushes protected branches)"
  fi

  if printf '%s' "$cmd" | grep -qE '(^|[;&|([:space:]])rm([[:space:]]+-[A-Za-z]*r[A-Za-z]*f|[[:space:]]+-[A-Za-z]*f[A-Za-z]*r|[[:space:]]+-[rf][[:space:]]+-[rf])'; then
    rm_safe=1
    for tok in $cmd; do
      tok="${tok%\"}"; tok="${tok#\"}"; tok="${tok%\'}"; tok="${tok#\'}"
      case "$tok" in
        "~/"*|"~") rm_safe=0 ;;
        /workspace|/workspace/*|/tmp|/tmp/*) ;;
        /*) rm_safe=0 ;;
      esac
    done
    [ "$rm_safe" -eq 1 ] || deny "rm -rf outside /workspace and /tmp"
  fi

  case "$cmd" in
    *consent-gate.sh*|*protected-paths*) deny "tampering with the consent gate" ;;
    *mirabilis-consent-approved*) deny "self-approving the consent gate (only the user may approve, via ! touch)" ;;
  esac
  printf '%s' "$cmd" | grep -qE '(plugin[[:space:]]+(remove|uninstall|disable)[[:space:]]+.*neuro-matrix|neuro-matrix.*(remove|uninstall|disable))' && deny "disabling the neuro-matrix harness"

  if printf '%s' "$cmd" | grep -qE '(\.credentials\.json|\.config/gh|CLAUDE_CODE_OAUTH_TOKEN|GITHUB_TOKEN|CONTEXT7_API_KEY|gh[[:space:]]+auth[[:space:]]+token|security[[:space:]]+find-generic-password)'; then
    printf '%s' "$cmd" | grep -qE '(curl|wget|nc[[:space:]]|netcat|scp|/dev/tcp|[[:space:]]mail[[:space:]]|nslookup|[[:space:]]dig[[:space:]])' && deny "exfiltrating credentials"
  fi

  printf '%s' "$cmd" | grep -qE '(^|[;&|([:space:]])sudo([[:space:]]|$)' && gate_request "Bash runs sudo: $cmd"
  printf '%s' "$cmd" | grep -qE '(^|[;&|([:space:]])(apt|apt-get)[[:space:]]+(install|remove|purge|upgrade|update|autoremove)([[:space:]]|$)' && gate_request "Bash modifies system packages: $cmd"

  has_write=0
  printf '%s' "$cmd" | grep -qE '(>>?|[[:space:]](tee|cp|mv|install|mkdir|rmdir|chmod|chown|ln|touch|dd)[[:space:]]|sed[[:space:]]+-i)' && has_write=1
  if [ "$has_write" -eq 1 ]; then
    for tok in $cmd; do
      tok="${tok%\"}"; tok="${tok#\"}"; tok="${tok%\'}"; tok="${tok#\'}"
      case "$tok" in "~/"*) tok="$HOME/${tok#\~/}" ;; esac
      case "${tok##*/}" in "$marker_base") deny "self-approving the consent gate (only the user may approve, via ! touch)" ;; esac
      case "$tok" in /*) is_protected "$tok" && gate_request "Bash writes into a protected path: $cmd" ;; esac
    done
  fi
  exit 0
fi

case "$tool" in
  Write|Edit|MultiEdit|NotebookEdit)
    path="$(printf '%s' "$input" | jq -r '.tool_input.file_path // .tool_input.notebook_path // ""' 2>/dev/null || echo "")"
    [ -n "$path" ] || exit 0
    case "$path" in
      *mirabilis-consent-approved*) deny "self-approving the consent gate (only the user may approve, via ! touch)" ;;
    esac
    is_protected "$path" && gate_request "$tool edits protected path: $path"
    exit 0
    ;;
  *) exit 0 ;;
esac
