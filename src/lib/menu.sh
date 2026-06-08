#!/usr/bin/env bash

do_launch() {
  first_run_stacks
  prepare_container "$(container_version)"
  first_run_setup
  preflight_gate
  local ght
  ght="$(dxq gh auth token || true)"
  [ -n "$ght" ] || echo "mirabilis: WARN — no GitHub token available; gh and the GitHub MCP may be limited." >&2
  exec "$DC" exec --workspace-folder "$REPO" env GITHUB_PERSONAL_ACCESS_TOKEN="$ght" COLORTERM=truecolor TERM=xterm-256color claude --dangerously-skip-permissions --append-system-prompt-file "$(system_prompt_file)"
}

do_update() {
  ensure_docker
  local behind
  behind="$(repo_behind)"
  if [ "${behind:-0}" -gt 0 ]; then
    echo "mirabilis: $behind commit(s) behind origin/main — updating source…" >&2
    pull_latest || echo "mirabilis: rebuilding the current source instead." >&2
  else
    echo "mirabilis: source is up to date — rebuilding the workspace…" >&2
  fi
  rebuild_image
  dc_up
  echo "mirabilis: updated to $(repo_version)." >&2
}

do_secrets() {
  ensure_docker
  container_running || dc_up
  ensure_github
  ensure_claude
  [ -x "$TOKEN" ] || return 0
  local chosen id
  chosen="$("$(menu_bin)" secrets)" || return 0
  [ -n "$chosen" ] || return 0
  printf '%s\n' "$chosen" | while IFS= read -r id; do
    [ -n "$id" ] || continue
    "$TOKEN" set "$id" </dev/tty || true
  done
}

do_theme() {
  ensure_docker
  container_running || dc_up
  local cur th
  cur="$(dxq bash -lc 'cat "$HOME/.claude/.mirabilis-theme" 2>/dev/null' || true)"
  [ -n "$cur" ] || cur=auto
  th="$("$(menu_bin)" theme --current "$cur")" || return 0
  set_theme "$th"
}

do_harness() {
  ensure_docker
  container_running || dc_up
  local cur sel
  cur="$(dxq bash -lc 'cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null' || true)"
  [ "$cur" = skip ] && cur=off || cur=on
  sel="$("$(menu_bin)" harness --current "$cur")" || return 0
  case "$sel" in
    off)
      dx bash -lc 'echo skip > "$HOME/.claude/.mirabilis-harness"' || true
      echo "mirabilis: harness will be OFF at next start." >&2
      ;;
    on)
      dx bash -lc 'echo install > "$HOME/.claude/.mirabilis-harness"' || true
      echo "mirabilis: harness will be ON at next start." >&2
      ;;
    reinstall) harness_reinstall ;;
    *) return 0 ;;
  esac
}

harness_reinstall() {
  dx bash -lc 'echo install > "$HOME/.claude/.mirabilis-harness"' || true
  echo "mirabilis: reinstalling neuro-matrix only (nothing else is touched)…" >&2
  if dxq bash /usr/local/bin/harness-reinstall.sh; then
    echo "mirabilis: neuro-matrix reinstalled." >&2
  else
    echo "mirabilis: WARN — harness reinstall reported a problem (check git/network/token)." >&2
  fi
}

do_plugins() {
  ensure_docker
  container_running || dc_up
  local catalog_csv enabled_csv chosen
  catalog_csv="$(dxq bash -lc 'sed -e "/^#/d" -e "/^[[:space:]]*\$/d" /opt/mirabilis/config/plugins.txt 2>/dev/null | tr "\n" "," | sed "s/,*$//"')"
  [ -n "$catalog_csv" ] || {
    echo "mirabilis: no plugin catalog found." >&2
    return 0
  }
  enabled_csv="$(dxq bash -lc '
    cat_all="$(sed -e "/^#/d" -e "/^[[:space:]]*\$/d" /opt/mirabilis/config/plugins.txt 2>/dev/null)"
    dis="$(cat "$HOME/.claude/.mirabilis-plugins-disabled" 2>/dev/null)"
    printf "%s\n" "$cat_all" | while IFS= read -r p; do
      printf "%s\n" "$dis" | grep -qxF "$p" || printf "%s," "$p"
    done | sed "s/,*$//"
  ' || true)"
  chosen="$("$(menu_bin)" plugins --options "$catalog_csv" --selected "$enabled_csv")" || return 0
  if dx env MCAT="$catalog_csv" MCHOSEN="$chosen" bash -lc '
    tmp="$(mktemp)"
    printf "%s" "$MCAT" | tr "," "\n" | while IFS= read -r p; do
      [ -n "$p" ] || continue
      printf "%s\n" "$MCHOSEN" | tr "," "\n" | grep -qxF "$p" || printf "%s\n" "$p"
    done >"$tmp" && mv "$tmp" "$HOME/.claude/.mirabilis-plugins-disabled" || { rm -f "$tmp"; exit 1; }
  '; then
    echo "mirabilis: plugin selection saved (applied at next start)." >&2
  else
    echo "mirabilis: WARN — could not save plugin selection; previous selection preserved." >&2
  fi
}

stacks_catalog() { sed -e '/^#/d' -e '/^[[:space:]]*$/d' "$REPO/config/stacks.txt" 2>/dev/null; }
stacks_current() { [ -f "$REPO/.env" ] && sed -n 's/^STACKS=//p' "$REPO/.env" | tail -n1; }
stacks_save() {
  local f="$REPO/.env" tmp line
  tmp="$(mktemp)"
  if [ -f "$f" ]; then
    while IFS= read -r line || [ -n "$line" ]; do
      case "$line" in STACKS=*) ;; *) printf '%s\n' "$line" >>"$tmp" ;; esac
    done <"$f"
  fi
  printf 'STACKS=%s\n' "$1" >>"$tmp"
  mv "$tmp" "$f"
}

select_stacks() {
  local catalog_csv current chosen
  catalog_csv="$(stacks_catalog | tr '\n' ',' | sed 's/,*$//')"
  [ -n "$catalog_csv" ] || {
    echo "mirabilis: no stack catalog found." >&2
    return 1
  }
  current="$(stacks_current)"
  chosen="$("$(menu_bin)" stacks --options "$catalog_csv" --selected "${current:-}")" || return 1
  stacks_save "$chosen"
}

do_stacks() {
  ensure_docker
  local before after
  before="$(stacks_current)"
  select_stacks || return 0
  after="$(stacks_current)"
  if [ "$before" = "$after" ]; then
    echo "mirabilis: stack selection unchanged." >&2
  else
    echo "mirabilis: stack changed → ${after:-none} — workspace marked stale; it rebuilds on next launch." >&2
  fi
}

resolve_code() {
  if command -v code >/dev/null 2>&1; then
    command -v code
    return 0
  fi
  local b
  for b in \
    "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" \
    "$HOME/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code" \
    "/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code"; do
    [ -x "$b" ] && {
      printf '%s' "$b"
      return 0
    }
  done
  return 1
}

offer_code_on_path() {
  local src="$1" dir
  for dir in ${MIRABILIS_BIN_DIRS:-/opt/homebrew/bin /usr/local/bin}; do
    if [ -d "$dir" ] && [ -w "$dir" ] && [ ! -e "$dir/code" ]; then
      ln -sf "$src" "$dir/code" 2>/dev/null &&
        echo "mirabilis: linked 'code' onto your PATH ($dir/code) — set up for next time." >&2
      return 0
    fi
  done
}

do_vscode() {
  ensure_docker
  local code_bin
  code_bin="$(resolve_code)" ||
    die "VS Code not found — install it from https://code.visualstudio.com, then run mirabilis again."
  case "$code_bin" in */Contents/Resources/app/bin/code) offer_code_on_path "$code_bin" ;; esac
  container_running || {
    echo "mirabilis: workspace is not running — starting it…" >&2
    dc_up
  }
  local name hex uri
  name="$(compose_container_name)"
  [ -n "$name" ] || die "could not determine the container name from docker-compose.yml."
  hex="$(printf '{"containerName":"/%s"}' "$name" | od -An -tx1 | tr -d ' \n')"
  uri="vscode-remote://attached-container+${hex}/workspace"
  echo "mirabilis: opening /workspace in VS Code (attached to container '$name')…" >&2
  "$code_bin" --folder-uri "$uri" || die "VS Code failed to open — is the Dev Containers extension installed?"
}

compose_container_name() {
  sed -n 's/^[[:space:]]*container_name:[[:space:]]*//p' "$REPO/docker-compose.yml" | head -n1 | tr -d '"'"'"' \t\r'
}

first_run_stacks() {
  [ -f "$REPO/.env" ] && grep -q '^STACKS=' "$REPO/.env" && return 0
  echo "mirabilis: первый запуск — выбери опциональные стеки (node + python + go уже в базе)." >&2
  select_stacks || stacks_save ""
}

first_run_setup() {
  dxq bash -lc 'test -f "$HOME/.claude/.mirabilis-setup-done"' && return 0
  echo "mirabilis: первый запуск — выбери, что предустановить." >&2
  do_harness
  do_plugins
  dx bash -lc 'touch "$HOME/.claude/.mirabilis-setup-done"' || true
}

menu() {
  ensure_tools
  ensure_docker
  local status choice
  while true; do
    status="$(menu_status_json)"
    choice="$(printf '%s' "$status" | "$(menu_bin)")" || choice=quit
    case "$choice" in
      update) do_update ;;
      plugins) do_plugins ;;
      harness) do_harness ;;
      stacks) do_stacks ;;
      vscode) do_vscode ;;
      secrets) do_secrets ;;
      theme) do_theme ;;
      quit) exit 0 ;;
      launch | *)
        do_launch
        exit $?
        ;;
    esac
  done
}

menu_status_json() {
  local behind nm hc harness_val stale
  behind="$(repo_behind)"
  case "$behind" in *[!0-9]*) behind=0 ;; esac
  hc="$(dxq bash -lc 'cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null' || true)"
  nm="$(nm_status)"
  if [ "$hc" = skip ]; then
    harness_val=off
  elif ! container_running; then
    harness_val=unknown
  elif [ "$nm" = missing ]; then
    harness_val=missing
  else
    harness_val=on
  fi
  if container_exists && is_stale; then stale=true; else stale=false; fi
  jq -n \
    --argjson commitsBehind "${behind:-0}" \
    --argjson stale "$stale" \
    --arg harness "$harness_val" \
    '{"commitsBehind":$commitsBehind,"stale":$stale,"harness":$harness}'
}

print_completion() {
  case "${1:-zsh}" in
    zsh) cat "$REPO/src/completions/_mirabilis" ;;
    *) die "only zsh completion is bundled" ;;
  esac
}
