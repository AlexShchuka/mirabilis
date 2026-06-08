setup() {
  load 'test_helper/common'
  _load_libs
  setup_shim_dir
  export SHIM_LOG="$BATS_TEST_TMPDIR/calls.log"
  : >"$SHIM_LOG"
  SCRIPT="$REPO_ROOT/src/git-identity.sh"
}

@test "derives name and email from gh api user" {
  make_shim gh 'echo "{\"login\":\"alex\",\"name\":\"Alex S\",\"email\":\"a@x.io\",\"id\":42}"'
  log_shim git 'exit 0'
  run bash "$SCRIPT"
  assert_success
  run cat "$SHIM_LOG"
  assert_line --partial 'config --global user.name Alex S'
  assert_line --partial 'config --global user.email a@x.io'
}

@test "falls back to noreply email when email is null" {
  make_shim gh 'echo "{\"login\":\"alex\",\"name\":null,\"email\":null,\"id\":42}"'
  log_shim git 'exit 0'
  run bash "$SCRIPT"
  assert_success
  run cat "$SHIM_LOG"
  assert_line --partial 'config --global user.name alex'
  assert_line --partial 'config --global user.email 42+alex@users.noreply.github.com'
}

@test "exits 0 silently when gh is missing" {
  rm -f "$SHIM_DIR/gh"
  PATH="$SHIM_DIR" run "$(command -v bash)" "$SCRIPT"
  assert_success
  run cat "$SHIM_LOG"
  refute_output --partial 'config'
}

@test "exits 0 when gh api user is empty" {
  make_shim gh 'exit 1'
  log_shim git 'exit 0'
  run bash "$SCRIPT"
  assert_success
  run cat "$SHIM_LOG"
  refute_output --partial 'config'
}
