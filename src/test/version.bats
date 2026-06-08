setup() {
  load 'test_helper/common'
  _load_libs
  setup_shim_dir
  REPO="$(mktemp -d "${BATS_TEST_TMPDIR}/repo.XXXXXX")"
  export REPO
  source_lib version
}

load 'lib_loader'

@test "repo_version: short sha from git" {
  make_shim git 'echo abc1234'
  run repo_version
  assert_success
  assert_output "abc1234"
}

@test "repo_version: unknown when git fails" {
  make_shim git 'exit 1'
  run repo_version
  assert_success
  assert_output "unknown"
}

@test "is_stale: true when container has no version" {
  make_shim git 'echo abc1234'
  make_shim docker ''
  run is_stale
  assert_success
}

@test "is_stale: false when repo version unknown" {
  make_shim git 'exit 1'
  make_shim docker 'echo MIRABILIS_VERSION=deadbee'
  run is_stale
  assert_failure
}

@test "is_stale: true when container version differs from repo" {
  make_shim git 'echo abc1234'
  make_shim docker 'echo MIRABILIS_VERSION=deadbee'
  run is_stale
  assert_success
}

@test "is_stale: false when versions match" {
  make_shim git 'echo abc1234'
  make_shim docker 'echo MIRABILIS_VERSION=abc1234'
  run is_stale
  assert_failure
}
