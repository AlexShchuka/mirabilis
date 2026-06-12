package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRoundtrip(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "claude token", key: "claude-token", value: "oat-abc-123"},
		{name: "telegram token", key: "telegram-token", value: "12345:bot-secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			store := NewFileStore(dir)
			ctx := context.Background()
			if err := store.Set(ctx, tt.key, tt.value); err != nil {
				t.Fatalf("Set: %v", err)
			}
			got, err := store.Get(ctx, tt.key)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got != tt.value {
				t.Errorf("Get = %q, want %q", got, tt.value)
			}
			info, err := os.Stat(filepath.Join(dir, ".mirabilis-"+tt.key))
			if err != nil {
				t.Fatalf("Stat: %v", err)
			}
			if perm := info.Mode().Perm(); perm != 0o600 {
				t.Errorf("perm = %v, want 0600", perm)
			}
		})
	}
}

func TestFileStoreMissing(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if _, err := store.Get(context.Background(), "claude-token"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get err = %v, want ErrNotFound", err)
	}
}

func TestFileStoreOverwriteEnforcesPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mirabilis-claude-token")
	if err := os.WriteFile(path, []byte("old-value"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	store := NewFileStore(dir)
	ctx := context.Background()
	if err := store.Set(ctx, "claude-token", "new-value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := store.Get(ctx, "claude-token")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "new-value" {
		t.Errorf("Get = %q, want %q", got, "new-value")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %v, want 0600", perm)
	}
}
