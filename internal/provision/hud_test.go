package provision

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func TestEnsureHudConfig_WritesWhenAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	seedDir := filepath.Join(tmp, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := `{"display":{"showCost":false}}`
	if err := os.WriteFile(filepath.Join(seedDir, "claude-hud.json"), []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New(seedDir)
	if err := EnsureHudConfig(cfg); err != nil {
		t.Fatalf("EnsureHudConfig: %v", err)
	}

	dest := filepath.Join(tmp, ".claude", "plugins", "claude-hud", "config.json")
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != want {
		t.Errorf("dest content = %q, want %q", got, want)
	}
}

func TestEnsureHudConfig_PreservesExisting(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	seedDir := filepath.Join(tmp, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "claude-hud.json"), []byte(`{"seed":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(tmp, ".claude", "plugins", "claude-hud", "config.json")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte(`{"user":"edit"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New(seedDir)
	if err := EnsureHudConfig(cfg); err != nil {
		t.Fatalf("EnsureHudConfig: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != `{"user":"edit"}` {
		t.Errorf("write-if-absent must not overwrite existing config, got %q", got)
	}
}

func TestEnsureHudConfig_NoSeedNoop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := config.New(filepath.Join(tmp, "seed"))
	if err := EnsureHudConfig(cfg); err != nil {
		t.Fatalf("EnsureHudConfig with no seed should be a noop, got %v", err)
	}
	dest := filepath.Join(tmp, ".claude", "plugins", "claude-hud", "config.json")
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("no dest should be created when seed is absent")
	}
}
