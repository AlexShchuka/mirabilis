#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${MIRABILIS_REPO_URL:-https://github.com/AlexShchuka/mirabilis.git}"
DEST="${MIRABILIS_HOME:-$HOME/.mirabilis}"

say() { printf 'mirabilis-install: %s\n' "$*" >&2; }
die() { printf 'mirabilis-install: %s\n' "$*" >&2; exit 1; }

[ "$(uname -s)" = Darwin ] || die "mirabilis targets macOS"
command -v git  >/dev/null 2>&1 || die "git is required — run: xcode-select --install"
command -v brew >/dev/null 2>&1 || die "Homebrew is required — install from https://brew.sh, then re-run"

if [ -d "$DEST/.git" ]; then
  say "updating the existing checkout at $DEST"
  git -C "$DEST" pull --ff-only || say "could not fast-forward — using the current checkout"
else
  say "cloning mirabilis to $DEST"
  git clone "$REPO_URL" "$DEST"
fi

say "installing prerequisites (Docker Desktop, devcontainer CLI, Go, tinyproxy)…"
make -C "$DEST" bootstrap
say "putting the 'mirabilis' command on your PATH…"
make -C "$DEST" install
say "done — run: mirabilis"
