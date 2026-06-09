#!/usr/bin/env bash

input=$(cat)
reset=$'\033[0m'
green=$'\033[32m'
red=$'\033[31m'

model=$(echo "$input" | jq -r '.model.display_name // "Unknown"')

branch=$(git -C "$(echo "$input" | jq -r '.workspace.current_dir')" rev-parse --abbrev-ref HEAD 2>/dev/null)
if [ -n "$branch" ]; then
  diff_stats=$(git -C "$(echo "$input" | jq -r '.workspace.current_dir')" diff --shortstat HEAD 2>/dev/null)
  added=$(echo "$diff_stats" | grep -oE '[0-9]+ insertion' | grep -oE '[0-9]+' || echo "0")
  deleted=$(echo "$diff_stats" | grep -oE '[0-9]+ deletion' | grep -oE '[0-9]+' || echo "0")
  [ -z "$added" ] && added=0
  [ -z "$deleted" ] && deleted=0
  git_info="🌿 ${branch} ${green}+${added}${reset}/${red}-${deleted}${reset}"
else
  git_info="🌿 no git"
fi

used_pct=$(echo "$input" | jq -r '.context_window.used_percentage // 0')
used_int=${used_pct%.*}
used_int=${used_int:-0}

bar_filled=$(( used_int / 5 ))
bar_empty=$(( 20 - bar_filled ))
bar=""
for ((i=0; i<bar_filled; i++)); do bar+="█"; done
for ((i=0; i<bar_empty; i++)); do bar+="░"; done

if [ "$used_int" -lt 50 ]; then
  bar_color=$'\033[32m'
elif [ "$used_int" -lt 80 ]; then
  bar_color=$'\033[33m'
else
  bar_color=$'\033[31m'
fi

context_bar="💭 ${bar_color}${bar}${reset} ${used_int}%"

cache_file="$HOME/.claude/ccusage-cache.json"
cache_max_age=300

needs_refresh=1
if [ -f "$cache_file" ]; then
  cache_mtime=$(stat -f %m "$cache_file" 2>/dev/null || stat -c %Y "$cache_file" 2>/dev/null)
  now=$(date +%s)
  if [ $(( now - cache_mtime )) -lt $cache_max_age ]; then
    needs_refresh=0
  fi
fi

if [ "$needs_refresh" -eq 1 ]; then
  ccusage_out=$(ccusage blocks --json 2>/dev/null)
  if [ -n "$ccusage_out" ] && echo "$ccusage_out" | jq . >/dev/null 2>&1; then
    echo "$ccusage_out" > "$cache_file"
  fi
fi

if [ -f "$cache_file" ]; then
  ccusage_out=$(cat "$cache_file")
  active_block=$(echo "$ccusage_out" | jq '[.blocks[] | select(.isActive == true)] | last')
  if [ -z "$active_block" ] || [ "$active_block" = "null" ]; then
    active_block=$(echo "$ccusage_out" | jq '.blocks | last')
  fi

  cost_usd=$(echo "$active_block" | jq -r '.costUSD // 0')
  cost_str=$(printf "💰 \$%.2f spent" "$cost_usd")

  end_time=$(echo "$active_block" | jq -r '.endTime // empty')
  if [ -n "$end_time" ]; then
    end_epoch=$(date -jf "%Y-%m-%dT%H:%M:%S" "${end_time%%.*}" +%s 2>/dev/null || \
                date -d "${end_time}" +%s 2>/dev/null || \
                echo "0")
    now_epoch=$(date +%s)
    diff_secs=$(( end_epoch - now_epoch ))
    if [ "$diff_secs" -gt 0 ]; then
      hours=$(( diff_secs / 3600 ))
      mins=$(( (diff_secs % 3600) / 60 ))
      time_str="⏱️  ${hours}h ${mins}m until reset"
    else
      time_str="⏱️  resetting…"
    fi
  else
    time_str="⏱️  —"
  fi
else
  cost_str="💰 —"
  time_str="⏱️  —"
fi

echo "🤖 ${model}   ${git_info}   ${context_bar}"
echo "${cost_str}   ${time_str}"
