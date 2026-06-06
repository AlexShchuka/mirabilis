#!/usr/bin/env bash
set -euo pipefail

command -v docker >/dev/null 2>&1 || { echo "[mirabilis] Docker is not installed — run 'make bootstrap'." >&2; exit 1; }
if ! docker info >/dev/null 2>&1 && [ -d /Applications/Docker.app ]; then
  echo "[mirabilis] starting Docker Desktop…" >&2
  open -a Docker
fi
