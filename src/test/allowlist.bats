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

@test "sandbox is enabled" {
  run jq -r '.sandbox.enabled' "$SETTINGS"
  assert_output "true"
}

@test "allowedDomains is a non-empty array" {
  run jq -e '.sandbox.network.allowedDomains | type == "array" and length > 0' "$SETTINGS"
  assert_success
}

@test "allowedDomains has no duplicates" {
  count="$(jq '.sandbox.network.allowedDomains | length' "$SETTINGS")"
  uniq="$(jq '.sandbox.network.allowedDomains | unique | length' "$SETTINGS")"
  assert_equal "$count" "$uniq"
}

@test "core domains are present" {
  for d in api.anthropic.com github.com registry.npmjs.org pypi.org api.telegram.org export.arxiv.org arxiv.org huggingface.co download.pytorch.org; do
    run jq -e --arg d "$d" '.sandbox.network.allowedDomains | index($d) != null' "$SETTINGS"
    assert_success
  done
}
