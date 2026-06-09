#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
    HOOK="$REPO_ROOT/.githooks/pre-commit"
    SANDBOX="$BATS_TEST_TMPDIR/repo"
    mkdir -p "$SANDBOX"
    git -C "$SANDBOX" init -q
    git -C "$SANDBOX" config user.email "t@t.com"
    git -C "$SANDBOX" config user.name "T"
}

@test "clean .sh staged exits 0" {
    printf 'exit 0\n' > "$SANDBOX/ok.sh"
    git -C "$SANDBOX" add ok.sh
    run sh -c "cd '$SANDBOX' && sh '$HOOK'"
    [ "$status" -eq 0 ]
}

@test ".sh with comment line exits 1 and names the file" {
    printf '#!/bin/sh\n# a comment\necho hi\n' > "$SANDBOX/bad.sh"
    git -C "$SANDBOX" add bad.sh
    run sh -c "cd '$SANDBOX' && sh '$HOOK'"
    [ "$status" -eq 1 ]
    [[ "$output" == *"bad.sh"* ]]
}

@test ".md with heading exits 0" {
    printf '# heading\ncontent\n' > "$SANDBOX/doc.md"
    git -C "$SANDBOX" add doc.md
    run sh -c "cd '$SANDBOX' && sh '$HOOK'"
    [ "$status" -eq 0 ]
}

@test "unformatted .go staged exits 1" {
    command -v go >/dev/null || skip "go not available"
    printf 'package main\nfunc main()  {\n}\n' > "$SANDBOX/bad.go"
    printf 'module sandbox\ngo 1.21\n' > "$SANDBOX/go.mod"
    git -C "$SANDBOX" add bad.go go.mod
    run sh -c "cd '$SANDBOX' && sh '$HOOK'"
    [ "$status" -eq 1 ]
}

@test "well-formatted .go staged exits 0" {
    command -v go >/dev/null || skip "go not available"
    printf 'package main\n\nfunc main() {}\n' > "$SANDBOX/ok.go"
    printf 'module sandbox\ngo 1.21\n' > "$SANDBOX/go.mod"
    git -C "$SANDBOX" add ok.go go.mod
    run sh -c "cd '$SANDBOX' && sh '$HOOK'"
    [ "$status" -eq 0 ]
}

@test "gitleaks absent yields stderr warn and exit 0" {
    command -v gitleaks >/dev/null && skip "gitleaks is present"
    printf 'exit 0\n' > "$SANDBOX/noleak.sh"
    git -C "$SANDBOX" add noleak.sh
    run sh -c "cd '$SANDBOX' && PATH='/usr/bin:/bin' sh '$HOOK'"
    [ "$status" -eq 0 ]
    [[ "$output" == *"gitleaks not found"* ]] || [[ "$stderr" == *"gitleaks not found"* ]]
}

@test "go vet failure exits 1" {
    command -v go >/dev/null || skip "go not available"
    printf 'package main\n\nimport "fmt"\n\nfunc main() { fmt.Printf("%%d", "not an int") }\n' > "$SANDBOX/vetbad.go"
    printf 'module sandbox\ngo 1.21\n' > "$SANDBOX/go.mod"
    git -C "$SANDBOX" add vetbad.go go.mod
    run sh -c "cd '$SANDBOX' && sh '$HOOK'"
    [ "$status" -eq 1 ]
}

@test "staging .githooks/pre-commit itself exits 0" {
    cp "$HOOK" "$SANDBOX/pre-commit"
    git -C "$SANDBOX" add pre-commit
    run sh -c "cd '$SANDBOX' && sh '$HOOK'"
    [ "$status" -eq 0 ]
}

@test ".github/workflows YAML with version comment exits 0" {
    mkdir -p "$SANDBOX/.github/workflows"
    comment="# v5.0.1"
    printf 'uses: actions/checkout@93cb6efe18208431cddfb8368fd83d5badbf9bfd %s\n' "$comment" > "$SANDBOX/.github/workflows/x.yml"
    git -C "$SANDBOX" add .github/workflows/x.yml
    run sh -c "cd '$SANDBOX' && sh '$HOOK'"
    [ "$status" -eq 0 ]
}
