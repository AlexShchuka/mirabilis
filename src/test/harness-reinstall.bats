setup() {
  load 'test_helper/common'
  _load_libs
  setup_shim_dir
  export SHIM_LOG="$BATS_TEST_TMPDIR/calls.log"
  : >"$SHIM_LOG"

  make_shim claude \
    'printf "claude %s\n" "$*" >> "$SHIM_LOG"' \
    'case "$*" in "plugin list") echo neuro-matrix ;; esac' \
    'exit 0'

  SCRIPT="$REPO_ROOT/src/harness-reinstall.sh"
}

@test "installs neuro-matrix from the upstream repo, not a local marketplace" {
  run bash "$SCRIPT"
  assert_success
  run cat "$SHIM_LOG"
  assert_line --partial 'plugin marketplace add AlexShchuka/neuro-matrix'
  assert_line --partial 'plugin install neuro-matrix@neuro-matrix --scope user'
  refute_output --partial '/opt/mirabilis/marketplace'
}

@test "exits 0 silently when claude is missing" {
  rm -f "$SHIM_DIR/claude"
  PATH="$SHIM_DIR" run "$(command -v bash)" "$SCRIPT"
  assert_success
  run cat "$SHIM_LOG"
  refute_output --partial 'plugin install'
}
