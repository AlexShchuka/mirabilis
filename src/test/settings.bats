setup() {
  load 'test_helper/common'
  _load_libs
  SETTINGS="$REPO_ROOT/config/settings.json"
  export SETTINGS
}

@test "settings.json is valid json" {
  run jq empty "$SETTINGS"
  assert_success
}

@test "no in-container sandbox block (egress is open, the container is the boundary)" {
  run jq -e '.sandbox == null' "$SETTINGS"
  assert_success
}

@test "statusLine is wired to the vendored statusline script" {
  run jq -e '.statusLine.command | test("statusline-command.sh")' "$SETTINGS"
  assert_success
}

@test "telegram notify hooks are present" {
  run jq -e '.hooks.Stop | type == "array" and length > 0' "$SETTINGS"
  assert_success
  run jq -e '.hooks.Notification | type == "array" and length > 0' "$SETTINGS"
  assert_success
}
