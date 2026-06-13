package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionKeyRoundTrip(t *testing.T) {
	repo := t.TempDir()
	const key = "deadbeefdeadbeefdeadbeefdeadbeef"

	if err := writeSessionKey(repo, key); err != nil {
		t.Fatalf("writeSessionKey: %v", err)
	}

	info, err := os.Stat(sessionKeyPath(repo))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("session-key perm = %o, want 0600", perm)
	}

	got, err := readSessionKey(repo)
	if err != nil {
		t.Fatalf("readSessionKey: %v", err)
	}
	if got != key {
		t.Fatalf("readSessionKey = %q, want %q", got, key)
	}
}

func TestWriteSessionKeyForcesPermOnOverwrite(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(sessionKeyPath(repo)), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sessionKeyPath(repo), []byte("stale"), 0o644); err != nil {
		t.Fatalf("preplant: %v", err)
	}

	if err := writeSessionKey(repo, "freshkeyfreshkeyfreshkeyfreshkey"); err != nil {
		t.Fatalf("writeSessionKey: %v", err)
	}

	info, err := os.Stat(sessionKeyPath(repo))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("session-key perm after overwrite = %o, want 0600", perm)
	}
}

func TestReadSessionKeyMissing(t *testing.T) {
	repo := t.TempDir()
	if _, err := readSessionKey(repo); err == nil {
		t.Fatal("readSessionKey on missing file = nil error, want error")
	}
}

func TestPromotedKeyReadsPersisted(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repo)
	const key = "cafebabecafebabecafebabecafebabe"
	if err := writeSessionKey(repo, key); err != nil {
		t.Fatalf("writeSessionKey: %v", err)
	}
	f, err := newFacade(repo)
	if err != nil {
		t.Fatalf("newFacade: %v", err)
	}
	t.Cleanup(func() { _ = f.obs.Close() })
	if got := promotedKey(f, repo); got != key {
		t.Fatalf("promotedKey = %q, want persisted %q", got, key)
	}
}

func TestPromotedKeyMissingReturnsEmpty(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repo)
	f, err := newFacade(repo)
	if err != nil {
		t.Fatalf("newFacade: %v", err)
	}
	t.Cleanup(func() { _ = f.obs.Close() })
	if got := promotedKey(f, repo); got != "" {
		t.Fatalf("promotedKey on missing file = %q, want empty (fresh-generate fallback)", got)
	}
}

func TestFacadeNewProxyPersistsAndReturnsKey(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("MIRABILIS_REPO", repo)
	f, err := newFacade(repo)
	if err != nil {
		t.Fatalf("newFacade: %v", err)
	}
	t.Cleanup(func() { _ = f.obs.Close() })

	const provided = "0123456789abcdef0123456789abcdef"
	p := f.newProxy(provided)
	if p.Key() != provided {
		t.Fatalf("proxy key = %q, want %q", p.Key(), provided)
	}
	if got := f.sessionKey(); got != provided {
		t.Fatalf("facade.sessionKey() = %q, want %q after newProxy", got, provided)
	}

	gen := f.newProxy("")
	if gen.Key() == "" || gen.Key() == provided {
		t.Fatalf("newProxy(\"\") key = %q, want fresh non-empty", gen.Key())
	}
	if f.sessionKey() != gen.Key() {
		t.Fatal("facade.sessionKey() not updated after fresh newProxy")
	}
}

func TestWriteSessionKeyTrimmedOnRead(t *testing.T) {
	repo := t.TempDir()
	if err := writeSessionKey(repo, "abc123\n"); err != nil {
		t.Fatalf("writeSessionKey: %v", err)
	}
	got, err := readSessionKey(repo)
	if err != nil {
		t.Fatalf("readSessionKey: %v", err)
	}
	if got != "abc123" {
		t.Fatalf("readSessionKey = %q, want trimmed %q", got, "abc123")
	}
}
