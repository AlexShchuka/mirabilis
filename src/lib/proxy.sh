#!/usr/bin/env bash

proxy_running() {
  [ -f "$PROXY_PID" ] || return 1
  local pid
  pid="$(cat "$PROXY_PID" 2>/dev/null)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

ensure_proxy() {
  command -v tinyproxy >/dev/null 2>&1 || die "tinyproxy is missing — run 'make bootstrap'."
  proxy_running && return 0
  printf 'Port %s\nListen 127.0.0.1\nTimeout 600\nAllow 127.0.0.1\nPidFile "%s"\nLogFile "%s"\n' \
    "$PROXY_PORT" "$PROXY_PID" "$PROXY_LOG" >"$PROXY_CONF"
  tinyproxy -c "$PROXY_CONF" >/dev/null 2>&1 || die "could not start the egress proxy (see $PROXY_LOG)."
}

stop_proxy() {
  if proxy_running; then kill "$(cat "$PROXY_PID" 2>/dev/null)" 2>/dev/null || true; fi
  rm -f "$PROXY_PID" 2>/dev/null || true
}
