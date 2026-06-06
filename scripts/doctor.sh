#!/usr/bin/env bash
set -uo pipefail
cd "$(dirname "$0")/.."

g=$'\033[32m'; r=$'\033[31m'; y=$'\033[33m'; z=$'\033[0m'
ok()   { printf '  %s\xe2\x9c\x93%s %s\n' "$g" "$z" "$*"; }
no()   { printf '  %s\xe2\x9c\x97%s %s\n' "$r" "$z" "$*"; }
warn() { printf '  %s!%s %s\n'           "$y" "$z" "$*"; }
DX()   { ./scripts/dc.sh exec --workspace-folder "$PWD" "$@" </dev/null 2>/dev/null; }

printf 'mirabilis doctor\n\nhost\n'
command -v docker >/dev/null 2>&1 && ok "docker present" || { no "docker missing (make bootstrap)"; exit 1; }
command -v devcontainer >/dev/null 2>&1 && ok "devcontainer CLI present" || no "devcontainer CLI missing (make bootstrap)"
docker info >/dev/null 2>&1 && ok "Docker daemon running" || { no "Docker daemon down (start Docker Desktop)"; exit 1; }
./scripts/token.sh check 2>&1 | sed 's/^/  /'

printf '\ncontainer\n'
if v="$(DX claude --version)"; then ok "claude $v"; else no "container not up (run: mirabilis)"; exit 0; fi
verc="$(DX bash -lc 'printf %s "${MIRABILIS_VERSION:-}"' | tr -d '[:space:]')"; vers="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
if [ -n "$verc" ] && [ "$verc" != unknown ] && [ "$vers" != unknown ] && [ "$verc" != "$vers" ]; then warn "container $verc behind source $vers (run: mirabilis update)"; else ok "version ${verc:-unknown}"; fi
sb="$(DX bash -lc 'jq -r ".sandbox.enabled" ~/.claude/settings.json 2>/dev/null')"
[ "$sb" = "true" ] && ok "sandbox egress allowlist active" || warn "sandbox not enabled in settings"
DX bash -lc 'jq -e ".enabledPlugins[\"neuro-matrix@mirabilis\"]" ~/.claude/settings.json >/dev/null 2>&1' \
  && ok "neuro-matrix plugin enabled" || warn "neuro-matrix not enabled (token?)"
DX claude mcp list 2>/dev/null | grep -qi github   && ok "github MCP"   || warn "github MCP not registered (needs token)"
DX claude mcp list 2>/dev/null | grep -qi context7 && ok "context7 MCP" || warn "context7 MCP not registered"

printf '\nnetwork\n'
if ip="$(DX curl -fsS --max-time 12 https://api.ipify.org)"; then ok "egress IP $ip"; else warn "no exit IP"; fi
code="$(DX curl -s -o /dev/null -w '%{http_code}' --max-time 15 https://api.anthropic.com/v1/models)"; code="${code:-000}"
case "$code" in
  200|401|403) ok "api.anthropic.com reachable ($code)" ;;
  000)         no "api.anthropic.com unreachable — check your connection" ;;
  *)           warn "api.anthropic.com $code" ;;
esac
printf '\ndone\n'
