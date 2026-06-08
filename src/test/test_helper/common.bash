_load_libs() {
  export BATS_LIB_PATH="${BATS_LIB_PATH:-/usr/lib:/usr/local/lib}"
  bats_load_library bats-support
  bats_load_library bats-assert
}

REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"
export REPO_ROOT

setup_shim_dir() {
  SHIM_DIR="$(mktemp -d "${BATS_TEST_TMPDIR:-${TMPDIR:-/tmp}}/shim.XXXXXX")"
  export SHIM_DIR
  export PATH="$SHIM_DIR:$PATH"
}

make_shim() {
  local name="$1"
  shift
  {
    printf '#!/usr/bin/env bash\n'
    printf '%s\n' "$@"
  } > "$SHIM_DIR/$name"
  chmod 0755 "$SHIM_DIR/$name"
}

log_shim() {
  local name="$1"
  shift
  {
    printf '#!/usr/bin/env bash\n'
    printf 'printf "%%s %%s\\n" "%s" "$*" >> "$SHIM_LOG"\n' "$name"
    printf '%s\n' "$@"
  } > "$SHIM_DIR/$name"
  chmod 0755 "$SHIM_DIR/$name"
}
