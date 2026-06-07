#!/usr/bin/env bash

die() { echo "mirabilis: $*" >&2; exit 1; }
have_gum() { command -v gum >/dev/null 2>&1; }
open_host_url() {
  if command -v open >/dev/null 2>&1; then open "$1" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then xdg-open "$1" >/dev/null 2>&1 || true
  fi
}

confirm() {
  local reply
  [ -t 0 ] || return 1
  if have_gum; then
    gum confirm "$1" --default=false
    return
  fi
  printf '%s [y/N] ' "$1" >/dev/tty 2>/dev/null || return 1
  read -r reply </dev/tty 2>/dev/null || return 1
  case "$reply" in [yY]|[yY][eE][sS]) return 0 ;; *) return 1 ;; esac
}
