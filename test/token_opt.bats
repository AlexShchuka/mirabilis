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

@test "headroom startHeadroom script sets ANTHROPIC_TARGET_API_URL from upstream" {
  grep -rq 'ANTHROPIC_TARGET_API_URL' "$REPO_ROOT/internal/hooks/"
}

@test "headroom proxy is started without --no-optimize (compression enabled by default)" {
  grep -q 'proxy ' "$REPO_ROOT/internal/hooks/session.go"
  ! grep -q 'no-optimize' "$REPO_ROOT/internal/hooks/session.go"
}

@test "caveman skill is in the skills catalog" {
  grep -q 'juliusbrussee/caveman' "$REPO_ROOT/config/skills.txt"
}

@test "auth chain: upstream file path is consistent between hooks and provision" {
  count=$(grep -r 'UpstreamFileName' "$REPO_ROOT/internal/" | grep -v '_test.go' | wc -l)
  [ "$count" -ge 2 ]
}

@test "I1 guard: token never injected via env-var hard-code in production source" {
  ! grep -rn 'ANTHROPIC_API_KEY\s*=\s*"sk-ant' \
    "$REPO_ROOT/internal/" "$REPO_ROOT/cmd/"
}
