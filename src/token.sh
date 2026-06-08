#!/usr/bin/env bash
set -euo pipefail

SERVICE_PREFIX="mirabilis"
ACCOUNT="${MIRABILIS_KEYCHAIN_ACCOUNT:-${USER:-mirabilis}}"
NAMES="gh claude context7 telegram-token telegram-chat"

svc() { printf '%s-%s-token' "$SERVICE_PREFIX" "$1"; }
envv() {
  case "$1" in
    gh) echo GITHUB_TOKEN ;;
    claude) echo CLAUDE_CODE_OAUTH_TOKEN ;;
    context7) echo CONTEXT7_API_KEY ;;
    telegram-token) echo TELEGRAM_BOT_TOKEN ;;
    telegram-chat) echo TELEGRAM_CHAT_ID ;;
    *) echo "" ;;
  esac
}

is_macos() { [[ "$(uname -s)" == "Darwin" ]]; }
die() {
  printf 'token.sh: %s\n' "$1" >&2
  exit 1
}
valid_name() { case "$1" in gh | claude | context7 | telegram-token | telegram-chat) ;; *) die "unknown secret '$1' (use: gh | claude | context7 | telegram-token | telegram-chat)" ;; esac }
kc_has() { security find-generic-password -a "$ACCOUNT" -s "$(svc "$1")" -w >/dev/null 2>&1; }

cmd_set() {
  local name="${1:-}"
  valid_name "$name"
  is_macos || die "set targets the macOS Keychain; on another OS, set the $(envv "$name") environment variable instead"
  local token=""
  if [ -t 0 ]; then
    printf 'Paste %s token (input hidden), then press Enter: ' "$name" >&2
    if ! IFS= read -rs token </dev/tty; then
      printf '\n' >&2
      die "no terminal available to read token (run interactively)"
    fi
    printf '\n' >&2
  else
    IFS= read -r token || true
  fi
  [[ -n "$token" ]] || die "empty input — nothing stored"
  security add-generic-password -a "$ACCOUNT" -s "$(svc "$name")" -w "$token" -U >/dev/null
  printf 'stored %s token in login Keychain (service=%s)\n' "$name" "$(svc "$name")" >&2
}

cmd_get() {
  local name="${1:-}"
  valid_name "$name"
  if is_macos && kc_has "$name"; then
    security find-generic-password -a "$ACCOUNT" -s "$(svc "$name")" -w
    return 0
  fi
  local ev
  ev="$(envv "$name")"
  if [[ -n "$ev" && -n "${!ev:-}" ]]; then
    printf '%s\n' "${!ev}"
    return 0
  fi
  die "no $name token found (run: ./src/token.sh set $name)"
}

cmd_rm() {
  local name="${1:-}"
  valid_name "$name"
  is_macos || die "rm targets the macOS Keychain only"
  if security delete-generic-password -a "$ACCOUNT" -s "$(svc "$name")" >/dev/null 2>&1; then
    printf 'removed %s token from Keychain\n' "$name" >&2
  else
    printf 'no %s token in Keychain\n' "$name" >&2
  fi
}

cmd_check() {
  local name ev val present
  for name in $NAMES; do
    ev="$(envv "$name")"
    val=""
    [[ -n "$ev" ]] && val="${!ev:-}"
    if is_macos && kc_has "$name"; then
      present="keychain"
    elif [[ -n "$val" ]]; then
      present="env ($ev)"
    else
      present="MISSING (run: token.sh set $name)"
    fi
    printf '%-15s %s\n' "$name" "$present"
  done
}

main() {
  local action="${1:-}"
  shift || true
  case "$action" in
    set) cmd_set "${1:-}" ;;
    get) cmd_get "${1:-}" ;;
    rm) cmd_rm "${1:-}" ;;
    check) cmd_check ;;
    *) die "usage: token.sh {set|get|rm} {gh|claude|context7|telegram-token|telegram-chat} | token.sh check" ;;
  esac
}

main "$@"
