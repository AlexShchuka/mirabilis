#!/usr/bin/env bash
set -uo pipefail
cd "$(dirname "$0")/.."

green=$'\033[32m'; red=$'\033[31m'; yellow=$'\033[33m'; reset=$'\033[0m'
ok()   { printf '  %s\xe2\x9c\x93%s %s\n' "$green"  "$reset" "$*"; }
no()   { printf '  %s\xe2\x9c\x97%s %s\n' "$red"    "$reset" "$*"; }
warn() { printf '  %s!%s %s\n'           "$yellow" "$reset" "$*"; }

cexec() { ./scripts/compose.sh exec -T --user coder -w /workspace mirabilis "$@" 2>/dev/null; }

printf 'mirabilis doctor\n\nhost\n'
if command -v docker >/dev/null 2>&1; then ok "docker present"; else no "docker missing (make bootstrap)"; exit 1; fi
if docker info >/dev/null 2>&1; then ok "Docker daemon running"; else no "Docker daemon down (open Docker Desktop)"; exit 1; fi
./scripts/token.sh check 2>&1 | sed 's/^/  /'

printf '\ncontainer\n'
if ./scripts/compose.sh ps --status running 2>/dev/null | grep -q mirabilis; then ok "container running"; else no "container not running (make up)"; exit 1; fi
if v="$(cexec claude --version)"; then ok "claude $v"; else no "claude --version failed"; fi

printf '\nnetwork / geo-exit\n'
if ip="$(cexec curl -fsS --max-time 12 https://api.ipify.org)"; then ok "egress IP $ip"; else warn "no exit IP (firewall/VPN?)"; fi
code="$(cexec curl -s -o /dev/null -w '%{http_code}' --max-time 15 https://api.anthropic.com/v1/models)"; code="${code:-000}"
case "$code" in
  200|401|403) ok "api.anthropic.com reachable (HTTP $code)" ;;
  000)         no "api.anthropic.com unreachable — VPN not inherited?" ;;
  *)           warn "api.anthropic.com HTTP $code" ;;
esac

printf '\nauth / plugin / mcp\n'
cexec gh auth status >/dev/null && ok "gh authenticated" || warn "gh not authenticated (make token-gh)"
cexec claude plugin list | grep -qi neuro-matrix && ok "neuro-matrix plugin present" || warn "neuro-matrix not found"
cexec claude mcp list | grep -qi github && ok "github MCP registered" || warn "github MCP not registered"
printf '\ndone\n'
