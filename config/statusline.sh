#!/bin/sh
input=$(cat)
model=$(printf '%s' "$input" | jq -r '.model.display_name // "?"')
dir=$(printf '%s' "$input" | jq -r '.workspace.current_dir // .cwd // ""')
ctx=$(printf '%s' "$input" | jq -r '.context_window.used_percentage // empty')
name=$(basename "$dir" 2>/dev/null)
branch=""
[ -n "$dir" ] && [ -d "$dir" ] && branch=$(git -C "$dir" branch --show-current 2>/dev/null)
line="[$model] $name"
[ -n "$branch" ] && line="$line | $branch"
[ -n "$ctx" ] && line="$line | ctx ${ctx}%"
printf '%s' "$line"
