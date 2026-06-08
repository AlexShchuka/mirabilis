setup() {
  load 'test_helper/common'
  _load_libs
  setup_shim_dir
  REPO="$REPO_ROOT"
  HOME="$BATS_TEST_TMPDIR/home"
  export REPO HOME
  mkdir -p "$HOME"
  source_lib util
  source_lib menu
}

load 'lib_loader'

@test "resolve_code: returns code from PATH when available" {
  make_shim code 'exit 0'
  run resolve_code
  assert_success
  assert_output --partial "/code"
}

@test "resolve_code: falls back to the VS Code app bundle" {
  bundle="$HOME/Applications/Visual Studio Code.app/Contents/Resources/app/bin"
  mkdir -p "$bundle"
  printf '#!/bin/sh\n' >"$bundle/code"
  chmod 0755 "$bundle/code"
  PATH="$SHIM_DIR:/usr/bin:/bin" run resolve_code
  assert_success
  assert_output --partial "Visual Studio Code.app"
}

@test "offer_code_on_path: links code into a writable bin dir that lacks it" {
  bindir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bindir"
  MIRABILIS_BIN_DIRS="$bindir" offer_code_on_path "/some/path/code"
  assert [ -L "$bindir/code" ]
}

@test "offer_code_on_path: leaves an existing code untouched" {
  bindir="$BATS_TEST_TMPDIR/bin"
  mkdir -p "$bindir"
  printf 'orig' >"$bindir/code"
  MIRABILIS_BIN_DIRS="$bindir" offer_code_on_path "/some/path/code"
  run cat "$bindir/code"
  assert_output "orig"
}
