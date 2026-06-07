#!/usr/bin/env bash

system_prompt_file() {
  dx bash -lc '
    sbx="$1"; out=/tmp/mirabilis-system-prompt.md
    nm="$HOME/.neuro-matrix/CLAUDE.md"
    if [ -f "$nm" ]; then cat "$sbx" "$nm" >"$out" && { printf %s "$out"; exit 0; }; fi
    printf "mirabilis: WARNING — neuro-matrix CLAUDE.md not found; using the sandbox note only\n" >&2
    printf %s "$sbx"
  ' _ "$SANDBOX_PROMPT"
}
