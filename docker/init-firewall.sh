#!/usr/bin/env bash
set -uo pipefail

log() { printf '[firewall] %s\n' "$*" >&2; }

DEFAULT_ALLOW=(
  api.anthropic.com
  claude.ai
  console.anthropic.com
  downloads.claude.ai
  statsig.anthropic.com
  api.githubcopilot.com
  github.com
  api.github.com
  codeload.github.com
  objects.githubusercontent.com
  raw.githubusercontent.com
  registry.npmjs.org
  api.ipify.org
  mcp.context7.com
  context7.com
)
read -r -a EXTRA_ALLOW <<< "${ALLOWLIST_EXTRA:-}"
ALLOW=("${DEFAULT_ALLOW[@]}" "${EXTRA_ALLOW[@]}")

if ! command -v iptables >/dev/null 2>&1; then
  log "iptables not available — EGRESS UNRESTRICTED"; exit 0
fi
if ! iptables -L >/dev/null 2>&1; then
  log "no NET_ADMIN/NET_RAW — EGRESS UNRESTRICTED (add cap_add: NET_ADMIN, NET_RAW)"; exit 0
fi

iptables -F
iptables -X 2>/dev/null || true
ipset destroy allowed-domains 2>/dev/null || true
ipset create allowed-domains hash:net family inet \
  || { log "FATAL: cannot create ipset — refusing to enable default-deny"; exit 1; }

iptables -A INPUT  -i lo -j ACCEPT
iptables -A OUTPUT -o lo -j ACCEPT
iptables -A INPUT  -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A OUTPUT -p udp --dport 53 -j ACCEPT
iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT

if meta="$(curl -fsS --max-time 10 https://api.github.com/meta 2>/dev/null)"; then
  while read -r cidr; do
    [[ -n "$cidr" ]] && ipset add allowed-domains "$cidr" -exist 2>/dev/null
  done < <(printf '%s' "$meta" | jq -r '((.web // []) + (.api // []) + (.git // []))[] | select(test(":") | not)')
  log "added GitHub meta IP ranges"
else
  log "WARN: could not fetch api.github.com/meta"
fi

for host in "${ALLOW[@]}"; do
  ips="$(getent ahostsv4 "$host" 2>/dev/null | awk '{print $1}' | sort -u)"
  if [[ -z "$ips" ]]; then log "WARN: could not resolve $host"; continue; fi
  while read -r ip; do
    [[ -n "$ip" ]] && ipset add allowed-domains "$ip" -exist 2>/dev/null
  done <<< "$ips"
  log "allow $host"
done

iptables -A OUTPUT -m set --match-set allowed-domains dst -j ACCEPT \
  || { log "FATAL: match-set rule failed — NOT enabling default-deny"; exit 1; }
if [[ "$(ipset list allowed-domains | grep -cE '^[0-9]')" -eq 0 ]]; then
  log "FATAL: allowed-domains is empty — NOT enabling default-deny"; exit 1
fi

iptables -P INPUT   DROP
iptables -P FORWARD DROP
iptables -P OUTPUT  DROP

ip6tables -F 2>/dev/null || true
ip6tables -X 2>/dev/null || true
ip6tables -A INPUT  -i lo -j ACCEPT 2>/dev/null || true
ip6tables -A OUTPUT -o lo -j ACCEPT 2>/dev/null || true
ip6tables -A INPUT  -m state --state ESTABLISHED,RELATED -j ACCEPT 2>/dev/null || true
ip6tables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT 2>/dev/null || true
ip6tables -A OUTPUT -p udp --dport 53 -j ACCEPT 2>/dev/null || true
ip6tables -A OUTPUT -p tcp --dport 53 -j ACCEPT 2>/dev/null || true
ip6tables -P INPUT   DROP 2>/dev/null || true
ip6tables -P FORWARD DROP 2>/dev/null || true
ip6tables -P OUTPUT  DROP 2>/dev/null || true

log "default-deny active ($(ipset list allowed-domains | grep -cE '^[0-9]') nets)"
