#!/usr/bin/env bats

exec_ok() {
  local probe="$1/.exec-probe.$$"
  printf '#!/bin/sh\nexit 0\n' >"$probe" 2>/dev/null || return 1
  chmod +x "$probe" 2>/dev/null || { rm -f "$probe"; return 1; }
  "$probe" >/dev/null 2>&1
  local rc=$?
  rm -f "$probe"
  return $rc
}

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  OS="$(uname -s)"
  if exec_ok "$BATS_TEST_TMPDIR"; then
    WORKDIR="$BATS_TEST_TMPDIR"
  else
    WORKDIR="$(mktemp -d "$REPO_ROOT/.bats-exec.XXXXXX")"
  fi
  BASEDIR="$WORKDIR/base"
  mkdir -p "$BASEDIR"
  for t in bash sh uname grep cat; do
    p="$(command -v "$t" 2>/dev/null)" && ln -sf "$p" "$BASEDIR/$t"
  done
}

teardown() {
  case "$WORKDIR" in
    "$REPO_ROOT"/.bats-exec.*) rm -rf "$WORKDIR" ;;
  esac
}

shim() {
  printf '#!/bin/sh\n%s\n' "$2" >"$BASEDIR/$1"
  chmod +x "$BASEDIR/$1"
}

is_wsl() {
  grep -qi microsoft /proc/version 2>/dev/null
}

@test "install.sh is syntactically valid bash" {
  run bash -n "$REPO_ROOT/install.sh"
  [ "$status" -eq 0 ]
}

@test "linux missing deps: prints all hints and exits non-zero" {
  [ "$OS" = Linux ] || skip "linux-only path"
  is_wsl && skip "covered by the WSL note test"
  shim apt-get 'exit 0'
  run env -i PATH="$BASEDIR" HOME="$WORKDIR/home" MIRABILIS_HOME="$WORKDIR/home" bash "$REPO_ROOT/install.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"git is required"* ]]
  [[ "$output" == *"make is required"* ]]
  [[ "$output" == *"go is required"* ]]
  [[ "$output" == *"node is required"* ]]
  [[ "$output" == *"npm is required"* ]]
  [[ "$output" == *"docker"* ]]
}

@test "linux all present: proceeds past checks" {
  [ "$OS" = Linux ] || skip "linux-only path"
  is_wsl && skip "covered by the WSL note test"
  for c in git make go node; do shim "$c" 'exit 0'; done
  shim docker 'case "$1" in compose) exit 0;; *) exit 0;; esac'
  shim npm 'exit 0'
  shim make 'exit 0'
  dest="$WORKDIR/home"
  mkdir -p "$dest/.git"
  run env -i PATH="$BASEDIR" HOME="$dest" MIRABILIS_HOME="$dest" bash "$REPO_ROOT/install.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"done — run: mirabilis"* ]]
}

@test "WSL: prints the Docker Desktop integration note" {
  is_wsl || skip "WSL-only path"
  for c in git make go node; do shim "$c" 'exit 0'; done
  shim docker 'exit 0'
  shim npm 'exit 0'
  shim make 'exit 0'
  dest="$WORKDIR/home"
  mkdir -p "$dest/.git"
  run env -i PATH="$BASEDIR" HOME="$dest" MIRABILIS_HOME="$dest" bash "$REPO_ROOT/install.sh"
  [[ "$output" == *"WSL detected"* ]]
}

@test "linux gitleaks absent: warns but still succeeds" {
  [ "$OS" = Linux ] || skip "linux-only path"
  is_wsl && skip "covered by the WSL note test"
  for c in git make go node; do shim "$c" 'exit 0'; done
  shim docker 'case "$1" in compose) exit 0;; *) exit 0;; esac'
  shim npm 'exit 0'
  shim make 'exit 0'
  dest="$WORKDIR/home"
  mkdir -p "$dest/.git"
  run env -i PATH="$BASEDIR" HOME="$dest" MIRABILIS_HOME="$dest" bash "$REPO_ROOT/install.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"gitleaks not found"* ]]
  [[ "$output" == *"done — run: mirabilis"* ]]
}

@test "darwin missing brew: refuses with a Homebrew hint" {
  [ "$OS" = Darwin ] || skip "darwin-only path"
  shim git 'exit 0'
  run env -i PATH="$BASEDIR" HOME="$WORKDIR/home" MIRABILIS_HOME="$WORKDIR/home" bash "$REPO_ROOT/install.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"Homebrew is required"* ]]
}

@test "darwin all present: runs bootstrap and install sequence" {
  [ "$OS" = Darwin ] || skip "darwin-only path"
  shim git 'exit 0'
  shim brew 'exit 0'
  shim make 'exit 0'
  dest="$WORKDIR/home"
  mkdir -p "$dest/.git"
  run env -i PATH="$BASEDIR" HOME="$dest" MIRABILIS_HOME="$dest" bash "$REPO_ROOT/install.sh"
  [ "$status" -eq 0 ]
  [[ "$output" == *"done — run: mirabilis"* ]]
}
