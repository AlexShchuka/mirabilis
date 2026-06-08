source_lib() {
  set +e
  set +u
  . "$REPO_ROOT/src/lib/$1.sh"
}
