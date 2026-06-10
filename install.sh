#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${MIRABILIS_REPO_URL:-https://github.com/AlexShchuka/mirabilis.git}"
DEST="${MIRABILIS_HOME:-$HOME/.mirabilis}"

say() { printf 'mirabilis-install: %s\n' "$*" >&2; }
die() {
  printf 'mirabilis-install: %s\n' "$*" >&2
  exit 1
}

is_wsl() { grep -qi microsoft /proc/version 2>/dev/null; }

pm_hint() {
  if command -v apt-get >/dev/null 2>&1; then
    printf 'sudo apt-get install -y %s' "$1"
  elif command -v dnf >/dev/null 2>&1; then
    printf 'sudo dnf install -y %s' "$1"
  elif command -v pacman >/dev/null 2>&1; then
    printf 'sudo pacman -S --noconfirm %s' "$1"
  else
    printf 'install %s with your package manager' "$1"
  fi
}

clone_and_finish() {
  if [ -d "$DEST/.git" ]; then
    say "updating the existing checkout at $DEST"
    git -C "$DEST" pull --ff-only || say "could not fast-forward — using the current checkout"
  else
    say "cloning mirabilis to $DEST"
    git clone "$REPO_URL" "$DEST"
  fi
  say "installing the devcontainer CLI…"
  npm install -g @devcontainers/cli
  say "putting the 'mirabilis' command on your PATH…"
  git -C "$DEST" config core.hooksPath .githooks
  make -C "$DEST" install
  say "done — run: mirabilis"
}

install_darwin() {
  command -v git >/dev/null 2>&1 || die "git is required — run: xcode-select --install"
  command -v brew >/dev/null 2>&1 || die "Homebrew is required — install from https://brew.sh, then re-run"

  if [ -d "$DEST/.git" ]; then
    say "updating the existing checkout at $DEST"
    git -C "$DEST" pull --ff-only || say "could not fast-forward — using the current checkout"
  else
    say "cloning mirabilis to $DEST"
    git clone "$REPO_URL" "$DEST"
  fi

  say "installing prerequisites (Docker Desktop, devcontainer CLI, Go)…"
  make -C "$DEST" bootstrap
  say "putting the 'mirabilis' command on your PATH…"
  make -C "$DEST" install
  say "done — run: mirabilis"
}

install_linux() {
  missing=0
  add_missing() {
    say "$1 is required — install with: $2"
    missing=1
  }

  command -v git >/dev/null 2>&1 || add_missing git "$(pm_hint git)"
  command -v make >/dev/null 2>&1 || add_missing make "$(pm_hint make)"
  command -v go >/dev/null 2>&1 || add_missing go "$(pm_hint golang)"
  command -v node >/dev/null 2>&1 || add_missing node "$(pm_hint nodejs)"
  command -v npm >/dev/null 2>&1 || add_missing npm "$(pm_hint npm)"
  if ! command -v docker >/dev/null 2>&1; then
    add_missing docker "curl -fsSL https://get.docker.com | sh"
  elif ! docker compose version >/dev/null 2>&1; then
    add_missing "docker compose v2" "$(pm_hint docker-compose-plugin)"
  fi

  if is_wsl; then
    say "WSL detected — if Docker is not reachable, enable Docker Desktop's WSL integration for this distro, or install docker-ce inside WSL"
  fi

  [ "$missing" -eq 0 ] || die "install the prerequisites above, then re-run this script"

  if ! command -v gitleaks >/dev/null 2>&1; then
    say "warning: gitleaks not found — the pre-commit hook will skip secret scanning; install with: go install github.com/zricethezav/gitleaks/v8@latest"
  fi

  clone_and_finish
}

case "$(uname -s)" in
  Darwin) install_darwin ;;
  Linux) install_linux ;;
  *) die "mirabilis targets macOS and Linux (including WSL2)" ;;
esac
