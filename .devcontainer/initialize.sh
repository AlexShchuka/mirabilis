#!/usr/bin/env bash
set -euo pipefail

command -v docker >/dev/null 2>&1 || { echo "[mirabilis] Docker is not installed — run 'make bootstrap'." >&2; exit 1; }
docker info >/dev/null 2>&1 || { echo "[mirabilis] Docker daemon is not running — start Docker Desktop, or run 'mirabilis' (it starts Docker for you)." >&2; exit 1; }
