#!/usr/bin/env bash

do_launch() {
  prepare_container "$(container_version)"
  first_run_setup
  preflight_gate
  local ght; ght="$(dxq gh auth token || true)"
  exec "$DC" exec --workspace-folder "$REPO" env GITHUB_PERSONAL_ACCESS_TOKEN="$ght" COLORTERM=truecolor TERM=xterm-256color claude --dangerously-skip-permissions --append-system-prompt-file "$(system_prompt_file)"
}

do_update() {
  ensure_docker
  local behind; behind="$(repo_behind)"
  if [ "${behind:-0}" -gt 0 ]; then
    echo "mirabilis: $behind commit(s) behind origin/main — updating source…" >&2
    pull_latest || echo "mirabilis: rebuilding the current source instead." >&2
  else
    echo "mirabilis: source is up to date — rebuilding the workspace…" >&2
  fi
  rebuild_image
  up
  echo "mirabilis: updated to $(repo_version)." >&2
}

do_secrets() {
  ensure_docker
  ensure_proxy
  container_running || up
  ensure_github
  ensure_claude
  if [ -x "$TOKEN" ]; then
    confirm "mirabilis: set/replace the context7 API key?" && "$TOKEN" set context7 || true
  fi
}

do_theme() {
  ensure_docker
  ensure_proxy
  container_running || up
  local th
  if have_gum; then
    th="$(gum choose --header "Theme" auto dark light dark-daltonized light-daltonized || true)"
  else
    printf 'theme (auto/dark/light): ' >/dev/tty 2>/dev/null || return 0
    read -r th </dev/tty 2>/dev/null || return 0
  fi
  set_theme "$th"
}

do_harness() {
  ensure_docker
  ensure_proxy
  container_running || up
  local cur sel
  cur="$(dxq bash -lc 'cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null' || echo install)"
  if have_gum; then
    sel="$(gum choose --header "neuro-matrix harness (сейчас: ${cur:-install})" "Установить" "Не устанавливать" "Обновить" || return 0)"
  else
    printf 'харнес (install/skip/update): ' >/dev/tty 2>/dev/null || return 0
    read -r sel </dev/tty 2>/dev/null || return 0
  fi
  case "$sel" in
    "Не устанавливать"|skip) dx bash -lc 'echo skip > "$HOME/.claude/.mirabilis-harness"' || true ;;
    "Обновить"|update)       dx bash -lc 'echo install > "$HOME/.claude/.mirabilis-harness"' || true ;;
    "Установить"|install)    dx bash -lc 'echo install > "$HOME/.claude/.mirabilis-harness"' || true ;;
    *)                       return 0 ;;
  esac
  dxq bash /opt/mirabilis/refresh.sh || true
  echo "mirabilis: harness setting saved." >&2
}

do_plugins() {
  ensure_docker
  ensure_proxy
  container_running || up
  have_gum || { echo "mirabilis: gum required for the plugin menu — run 'make bootstrap'." >&2; return 0; }
  local catalog preselect chosen dis line
  catalog="$(dxq bash -lc 'sed -e "/^#/d" -e "/^[[:space:]]*$/d" /opt/mirabilis/config/plugins.txt 2>/dev/null')"
  [ -n "$catalog" ] || { echo "mirabilis: no plugin catalog found." >&2; return 0; }
  preselect="$(printf '%s' "$catalog" | tr '\n' ',' | sed 's/,*$//')"
  chosen="$(printf '%s\n' "$catalog" | gum choose --no-limit --selected "$preselect" --header "Плагины (пробел — переключить, Enter — ок)" || true)"
  [ -n "$chosen" ] || chosen="$catalog"
  dis=""
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    printf '%s\n' "$chosen" | grep -qxF "$line" || dis="$dis$line"$'\n'
  done <<EOF
$catalog
EOF
  dx env MDIS="$dis" bash -lc 'printf "%s" "$MDIS" > "$HOME/.claude/.mirabilis-plugins-disabled"' || true
  dxq bash /opt/mirabilis/refresh.sh || true
  echo "mirabilis: plugin selection saved." >&2
}

first_run_setup() {
  have_gum || return 0
  dxq bash -lc 'test -f "$HOME/.claude/.mirabilis-setup-done"' && return 0
  echo "mirabilis: первый запуск — выбери, что предустановить." >&2
  do_harness
  do_plugins
  dx bash -lc 'touch "$HOME/.claude/.mirabilis-setup-done"' || true
}

menu() {
  ensure_tools
  ensure_extras
  ensure_docker
  local behind nm hc header choice
  behind="$(repo_behind)"
  nm="$(nm_status)"
  hc="$(dxq bash -lc 'cat "$HOME/.claude/.mirabilis-harness" 2>/dev/null' || true)"
  header="mirabilis"
  container_exists && is_stale && header="$header · workspace: stale (rebuild on launch)"
  [ "${behind:-0}" -gt 0 ] && header="$header · mirabilis: $behind behind origin/main"
  if [ "$hc" = skip ]; then header="$header · neuro-matrix: off"
  elif [ "$nm" = missing ]; then header="$header · neuro-matrix: missing"; fi
  if have_gum; then
    choice="$(gum choose --header "$header" "Запустить" "Обновить" "Плагины" "Харнес" "Войти / секреты" "Тема" "Выход" || echo Выход)"
  else
    echo "$header" >&2
    echo "  1) Запустить  2) Обновить  3) Плагины  4) Харнес  5) Войти / секреты  6) Тема  7) Выход" >&2
    printf 'выбор [1]: ' >/dev/tty 2>/dev/null || { do_launch; return; }
    local n; read -r n </dev/tty 2>/dev/null || n=1
    case "${n:-1}" in 2) choice=Обновить ;; 3) choice=Плагины ;; 4) choice=Харнес ;; 5) choice="Войти / секреты" ;; 6) choice=Тема ;; 7) choice=Выход ;; *) choice=Запустить ;; esac
  fi
  case "$choice" in
    "Обновить")          do_update; menu ;;
    "Плагины")           do_plugins; menu ;;
    "Харнес")            do_harness; menu ;;
    "Войти / секреты")   do_secrets; menu ;;
    "Тема")              do_theme; menu ;;
    "Выход")             exit 0 ;;
    *)                   do_launch ;;
  esac
}

print_completion() {
  case "${1:-zsh}" in
    zsh) cat "$REPO/src/completions/_mirabilis" ;;
    *)   die "only zsh completion is bundled" ;;
  esac
}
