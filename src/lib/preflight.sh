#!/usr/bin/env bash

preflight() {
  local host_ip cont_ip code sb mcp hc crit="" warn=""
  cont_ip="$(dxq curl -s -m 8 https://api.ipify.org || true)"
  host_ip="$(curl -s -m 8 https://api.ipify.org 2>/dev/null || true)"
  if [ -z "$cont_ip" ]; then
    crit="$crit"$'\n'"  egress: the container has no outbound — is the proxy up?"
  elif [ -n "$host_ip" ] && [ "$cont_ip" != "$host_ip" ]; then
    warn="$warn"$'\n'"  egress: container exits $cont_ip, host exits $host_ip — not routing through your host"
  fi
  code="$(dxq curl -s -o /dev/null -w '%{http_code}' -m 12 https://api.anthropic.com/v1/models || true)"
  case "${code:-000}" in
    200 | 401 | 403) ;;
    000) crit="$crit"$'\n'"  api.anthropic.com: unreachable" ;;
    *) crit="$crit"$'\n'"  api.anthropic.com: HTTP $code" ;;
  esac
  sb="$(dxq bash -lc 'jq -r ".sandbox.enabled" ~/.claude/settings.json 2>/dev/null' || true)"
  [ "$sb" = true ] || crit="$crit"$'\n'"  sandbox: not enabled in settings"
  hc="$(dxq bash -lc 'cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null' || true)"
  if [ "$hc" = skip ]; then
    warn="$warn"$'\n'"  neuro-matrix: harness disabled — running bare (no protocol)"
  else
    dxq bash -lc 'claude plugin list 2>/dev/null | grep -q neuro-matrix' ||
      warn="$warn"$'\n'"  neuro-matrix: harness selected but not installed (check git/network/token)"
  fi
  mcp="$(dxq claude mcp list || true)"
  printf '%s\n' "$mcp" | grep -qi github || warn="$warn"$'\n'"  github MCP: not registered (token?)"
  printf '%s\n' "$mcp" | grep -qi context7 || warn="$warn"$'\n'"  context7 MCP: not registered"
  printf '%s\037%s' "$crit" "$warn"
}

preflight_gate() {
  local out crit warn
  out="$(preflight)"
  crit="${out%%$'\037'*}"
  warn="${out#*$'\037'}"
  if [ -n "$crit" ]; then
    echo "mirabilis: critical checks failed — re-running per-start setup once…" >&2
    dxq bash /opt/mirabilis/refresh.sh ||
      echo "mirabilis: WARN — per-start setup retry itself failed (see container logs)" >&2
    out="$(preflight)"
    crit="${out%%$'\037'*}"
    warn="${out#*$'\037'}"
  fi
  [ -n "$warn" ] && printf 'mirabilis: warnings (continuing) —%s\n' "$warn" >&2
  if [ -n "$crit" ]; then
    printf 'mirabilis: STOP — critical environment failure:%s\n' "$crit" >&2
    die "egress and Claude access are required — fix the above and run mirabilis again"
  fi
  printf 'mirabilis: healthy — egress via your host\n' >&2
}
