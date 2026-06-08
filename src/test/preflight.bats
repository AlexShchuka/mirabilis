setup() {
  load 'test_helper/common'
  _load_libs
  setup_shim_dir
  source_lib preflight
}

load 'lib_loader'

stub_die() {
  die() {
    echo "DIE: $*" >&2
    return 1
  }
}

@test "preflight_gate: passes when no crit and no warn" {
  dxq() {
    case "$1 $2 $3" in
      *ipify*) echo "1.2.3.4" ;;
      *http_code*) echo "200" ;;
      *) echo true ;;
    esac
  }
  curl() { echo "1.2.3.4"; }
  preflight() { printf '%s\037%s' "" ""; }
  run preflight_gate
  assert_success
  assert_output --partial "healthy"
}

@test "preflight_gate: warns but continues" {
  preflight() { printf '%s\037%s' "" $'\n  github MCP: not registered'; }
  run preflight_gate
  assert_success
  assert_output --partial "warnings (continuing)"
}

@test "preflight_gate: stops on a critical failure" {
  stub_die
  dxq() { bash /opt/mirabilis/refresh.sh 2>/dev/null || true; }
  preflight() { printf '%s\037%s' $'\n  api.anthropic.com: unreachable' ""; }
  run preflight_gate
  assert_failure
  assert_output --partial "STOP"
}
