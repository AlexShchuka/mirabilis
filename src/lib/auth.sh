#!/usr/bin/env bash

ensure_github() {
  dx gh auth status >/dev/null 2>&1 && return 0
  echo "mirabilis: signing in to GitHub. Your Mac browser will open github.com/login/device; type the one-time code shown below and approve." >&2
  open_host_url "https://github.com/login/device"
  dx env GH_BROWSER=true BROWSER=true gh auth login --hostname github.com --git-protocol https --web || die "GitHub sign-in failed — run 'mirabilis' again"
  dx gh auth setup-git || true
  dx bash /usr/local/bin/git-identity.sh || true
}

ensure_claude() {
  dxq bash -lc '[ -s "$HOME/.claude/.credentials.json" ] || [ -n "${CLAUDE_CODE_OAUTH_TOKEN:-}" ]' && return 0
  echo "mirabilis: Claude needs sign-in. Complete the sign-in Claude shows on first launch: it prints a URL to open in your browser; approve, then paste the code back. Saved for next time." >&2
}

set_theme() {
  local th="$1"
  case "$th" in auto | dark | light | dark-daltonized | light-daltonized) ;; *) return 0 ;; esac
  dx bash -lc "printf '%s\n' '$th' > \"\$HOME/.claude/.mirabilis-theme\"" || true
  dx bash -lc "tmp=\$(mktemp); jq --arg t '$th' '.theme=\$t' \"\$HOME/.claude/settings.json\" > \"\$tmp\" && mv \"\$tmp\" \"\$HOME/.claude/settings.json\" || rm -f \"\$tmp\"" &&
    echo "mirabilis: theme set to $th." >&2 || echo "mirabilis: theme saved for next launch." >&2
}
