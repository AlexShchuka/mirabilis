#!/usr/bin/env bash

die() {
  echo "mirabilis: $*" >&2
  exit 1
}
open_host_url() { command -v open >/dev/null 2>&1 && open "$1" >/dev/null 2>&1 || true; }

menu_bin() {
  if [ -n "${MIRABILIS_MENU_BIN:-}" ] && [ -x "$MIRABILIS_MENU_BIN" ]; then
    printf '%s' "$MIRABILIS_MENU_BIN"
    return 0
  fi
  local b="$REPO/src/menu/bin/mirabilis-menu"
  [ -x "$b" ] || die "menu binary missing at $b — run 'make install' (builds the Go menu)."
  printf '%s' "$b"
}

confirm() {
  local reply
  [ -t 0 ] || return 1
  printf '%s [y/N] ' "$1" >/dev/tty 2>/dev/null || return 1
  read -r reply </dev/tty 2>/dev/null || return 1
  case "$reply" in [yY] | [yY][eE][sS]) return 0 ;; *) return 1 ;; esac
}
