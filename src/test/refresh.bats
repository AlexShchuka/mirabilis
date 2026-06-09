setup() {
  load 'test_helper/common'
  _load_libs
  setup_shim_dir
  export SHIM_LOG="$BATS_TEST_TMPDIR/calls.log"
  : >"$SHIM_LOG"

  FAKE_HOME="$BATS_TEST_TMPDIR/home"
  FAKE_OPT="$BATS_TEST_TMPDIR/opt/mirabilis"
  mkdir -p "$FAKE_HOME/.claude" "$FAKE_OPT/config"
  cp "$REPO_ROOT/config/settings.json" "$FAKE_OPT/config/settings.json"
  : >"$FAKE_OPT/config/apt-packages.txt"
  : >"$FAKE_OPT/config/plugins.txt"

  for c in sudo gh git dpkg apt-get visudo; do make_shim "$c" 'exit 0'; done
  log_shim claude 'exit 0'
  log_shim rtk 'exit 0'
  log_shim timeout 'shift' 'exec "$@"'
  make_shim provision-mcp.sh 'exit 0'
  make_shim git-identity.sh 'exit 0'

  REFRESH="$REPO_ROOT/.devcontainer/refresh.sh"
}

run_refresh() {
  HOME="$FAKE_HOME" \
    PATH="$SHIM_DIR:$PATH" \
    FAKE_OPT="$FAKE_OPT" \
    REFRESH="$REFRESH" \
    LOCAL="$BATS_TEST_TMPDIR/refresh.local.sh" \
    bash -c '
      set -uo pipefail
      sed -e "s#/opt/mirabilis#$FAKE_OPT#g" \
        -e "s#/usr/local/bin/provision-mcp.sh#provision-mcp.sh#g" \
        -e "s#/usr/local/bin/git-identity.sh#git-identity.sh#g" \
        -e "s#^export HOME=/home/node#export HOME=$HOME#g" \
        "$REFRESH" >"$LOCAL"
      bash "$LOCAL"
    '
}

@test "refresh seeds settings.json on first run" {
  run_refresh
  assert [ -f "$FAKE_HOME/.claude/settings.json" ]
  run jq -e '.sandbox == null' "$FAKE_HOME/.claude/settings.json"
  assert_success
}

@test "refresh is idempotent: settings.json stable across two runs" {
  run_refresh
  jq --sort-keys . "$FAKE_HOME/.claude/settings.json" >"$BATS_TEST_TMPDIR/first.json"
  run_refresh
  jq --sort-keys . "$FAKE_HOME/.claude/settings.json" >"$BATS_TEST_TMPDIR/second.json"
  run diff "$BATS_TEST_TMPDIR/first.json" "$BATS_TEST_TMPDIR/second.json"
  assert_success
}

@test "refresh does not install the neuro-matrix harness (owned by the launch step)" {
  run_refresh
  run cat "$SHIM_LOG"
  refute_output --partial 'install neuro-matrix'
  refute_output --partial 'marketplace add /opt/mirabilis/marketplace'
}

@test "refresh bounds rtk init with timeout" {
  run_refresh
  run cat "$SHIM_LOG"
  assert_line --regexp 'timeout [0-9]+ rtk init -g --auto-patch'
}
