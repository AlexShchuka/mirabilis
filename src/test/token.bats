setup() {
  load 'test_helper/common'
  _load_libs
  setup_shim_dir
  TOKEN="$REPO_ROOT/src/token.sh"
  export KC_STORE="$BATS_TEST_TMPDIR/kc"
  : >"$KC_STORE"
  make_shim uname 'echo Darwin'
  make_security_shim
}

make_security_shim() {
  cat >"$SHIM_DIR/security" <<'SH'
#!/usr/bin/env bash
sub="$1"; shift
svc=""; acct=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -s) svc="$2"; shift 2 ;;
    -a) acct="$2"; shift 2 ;;
    -w) if [ "$sub" = add-generic-password ]; then val="$2"; shift 2; else shift; fi ;;
    -U) shift ;;
    *) shift ;;
  esac
done
key="$acct/$svc"
case "$sub" in
  add-generic-password)
    grep -v "^$key	" "$KC_STORE" > "$KC_STORE.t" 2>/dev/null; mv "$KC_STORE.t" "$KC_STORE"
    printf '%s\t%s\n' "$key" "$val" >> "$KC_STORE" ;;
  find-generic-password)
    line="$(grep "^$key	" "$KC_STORE" 2>/dev/null)" || exit 44
    [ -n "$line" ] || exit 44
    printf '%s\n' "${line#*	}" ;;
  delete-generic-password)
    grep -q "^$key	" "$KC_STORE" 2>/dev/null || exit 44
    grep -v "^$key	" "$KC_STORE" > "$KC_STORE.t"; mv "$KC_STORE.t" "$KC_STORE" ;;
esac
SH
  chmod 0755 "$SHIM_DIR/security"
}

@test "set then get returns the stored token (non-tty path uses env)" {
  CONTEXT7_API_KEY=abc123 KC_STORE="$KC_STORE" run bash "$TOKEN" get context7
  assert_success
  assert_output "abc123"
}

@test "get fails for a name with no env and no keychain entry" {
  run bash "$TOKEN" get context7
  assert_failure
  assert_output --partial "no context7 token found"
}

@test "set writes to keychain and get reads it back" {
  printf 'sekret\n' | KC_STORE="$KC_STORE" run bash "$TOKEN" set context7
  KC_STORE="$KC_STORE" run bash "$TOKEN" get context7
  assert_success
  assert_output "sekret"
}

@test "rm removes the keychain entry" {
  printf 'sekret\n' | run bash "$TOKEN" set context7
  run bash "$TOKEN" rm context7
  assert_success
  run bash "$TOKEN" get context7
  assert_failure
}

@test "unknown secret name is rejected" {
  run bash "$TOKEN" get bogus
  assert_failure
  assert_output --partial "unknown secret"
}

@test "check lists all five names" {
  run bash "$TOKEN" check
  assert_success
  assert_line --partial "gh"
  assert_line --partial "claude"
  assert_line --partial "context7"
  assert_line --partial "telegram-token"
  assert_line --partial "telegram-chat"
}
