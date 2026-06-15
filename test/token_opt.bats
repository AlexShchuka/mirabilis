#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)"
}

@test "rtk-config.toml seed has tee failures mode" {
  grep -q 'mode = "failures"' "$REPO_ROOT/config/rtk-config.toml"
}

@test "rtk-config.toml seed has hooks exclude_commands" {
  grep -q 'exclude_commands' "$REPO_ROOT/config/rtk-config.toml"
}

@test "headroom --mode is threaded from config (CacheAligner default enabled)" {
  grep -q '\-\-mode' "$REPO_ROOT/internal/hooks/session.go"
  grep -q '\-\-mode' "$REPO_ROOT/internal/engine/provision/headroom.go"
  grep -q 'HeadroomMode' "$REPO_ROOT/internal/hooks/session.go"
  grep -q 'HeadroomMode' "$REPO_ROOT/internal/engine/provision/headroom.go"
}

@test "upstream reaches headroom via process env not shell interpolation (CWE-78 fix)" {
  ! grep -rn 'ANTHROPIC_TARGET_API_URL=%q' "$REPO_ROOT/internal/"
  grep -q 'ANTHROPIC_TARGET_API_URL.*upstream' "$REPO_ROOT/internal/hooks/session.go"
  grep -q 'ANTHROPIC_TARGET_API_URL.*upstream' "$REPO_ROOT/internal/engine/provision/headroom.go"
}

@test "headroom proxy is started without --no-optimize (compression enabled by default)" {
  ! grep -q 'no-optimize' "$REPO_ROOT/internal/hooks/session.go"
  ! grep -q 'no-optimize' "$REPO_ROOT/internal/engine/provision/headroom.go"
}

@test "caveman is in the plugin catalog" {
  grep -q 'caveman@caveman' "$REPO_ROOT/config/plugins.txt"
}

@test "caveman marketplace is registered" {
  grep -q 'JuliusBrussee/caveman' "$REPO_ROOT/config/marketplaces.txt"
}

@test "auth chain: upstream file path is consistent between hooks and provision" {
  count=$(grep -r 'UpstreamFileName' "$REPO_ROOT/internal/" | grep -v '_test.go' | wc -l)
  [ "$count" -ge 2 ]
}

@test "CWE-798 regression guard: no hardcoded Anthropic key in production source" {
  ! grep -rn 'ANTHROPIC_API_KEY[[:space:]]*=[[:space:]]*"sk-ant' \
    "$REPO_ROOT/internal/" "$REPO_ROOT/cmd/"
}
