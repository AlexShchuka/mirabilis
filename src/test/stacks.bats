setup() {
  load 'test_helper/common'
  _load_libs
}

@test "stack catalog offers dotnet, not go (go is baked into the base image)" {
  run cat "$REPO_ROOT/config/stacks.txt"
  assert_success
  assert_line "dotnet"
  refute_line "go"
}

@test "Dockerfile installs Go unconditionally (not behind a STACKS gate)" {
  run grep -q 'go.dev/dl' "$REPO_ROOT/docker/Dockerfile"
  assert_success
  run grep -q '[*],go,[*]' "$REPO_ROOT/docker/Dockerfile"
  assert_failure
}

@test "Dockerfile keeps dotnet as an optional STACKS stack" {
  run grep -q '[*],dotnet,[*]' "$REPO_ROOT/docker/Dockerfile"
  assert_success
}
