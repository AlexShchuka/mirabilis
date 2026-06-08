setup() {
  load 'test_helper/common'
  _load_libs
  setup_shim_dir

  FAKE_HOME="$BATS_TEST_TMPDIR/home"
  FAKE_OPT="$BATS_TEST_TMPDIR/opt/mirabilis"
  mkdir -p "$FAKE_HOME/.claude" "$FAKE_OPT/config" "$FAKE_OPT/marketplace/.claude-plugin"
  cp "$REPO_ROOT/config/settings.json" "$FAKE_OPT/config/settings.json"
  : >"$FAKE_OPT/config/apt-packages.txt"
  : >"$FAKE_OPT/config/plugins.txt"

  for c in sudo gh git claude rtk dpkg apt-get visudo; do make_shim "$c" 'exit 0'; done
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
  run jq -e '.sandbox.enabled == true' "$FAKE_HOME/.claude/settings.json"
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

@test "refresh trust dialog idempotent: .claude.json stable across two runs" {
  run_refresh
  jq --sort-keys . "$FAKE_HOME/.claude.json" >"$BATS_TEST_TMPDIR/first.cj" 2>/dev/null || skip ".claude.json not created"
  run_refresh
  jq --sort-keys . "$FAKE_HOME/.claude.json" >"$BATS_TEST_TMPDIR/second.cj"
  run diff "$BATS_TEST_TMPDIR/first.cj" "$BATS_TEST_TMPDIR/second.cj"
  assert_success
}
