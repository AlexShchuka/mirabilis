package provision

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func TestEnsureRTKConfig_WritesWhenAbsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")

	seedDir := filepath.Join(tmp, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := "[hooks]\nexclude_commands = [\"pytest\"]\n"
	if err := os.WriteFile(filepath.Join(seedDir, "rtk-config.toml"), []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New(seedDir)
	if err := EnsureRTKConfig(cfg); err != nil {
		t.Fatalf("EnsureRTKConfig: %v", err)
	}

	dest := filepath.Join(tmp, ".config", "rtk", "config.toml")
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != want {
		t.Errorf("dest content = %q, want %q", got, want)
	}
}

func TestEnsureRTKConfig_PreservesExisting(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")

	seedDir := filepath.Join(tmp, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "rtk-config.toml"), []byte("[hooks]\nexclude_commands = [\"pytest\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(tmp, ".config", "rtk", "config.toml")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "[hooks]\nexclude_commands = [\"jest\"]\n"
	if err := os.WriteFile(dest, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New(seedDir)
	if err := EnsureRTKConfig(cfg); err != nil {
		t.Fatalf("EnsureRTKConfig: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != existing {
		t.Errorf("write-if-absent must not overwrite existing config, got %q", got)
	}
}

func TestEnsureRTKConfig_NoSeedNoop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfg := config.New(filepath.Join(tmp, "seed"))
	if err := EnsureRTKConfig(cfg); err != nil {
		t.Fatalf("EnsureRTKConfig with no seed should be a noop, got %v", err)
	}
	dest := filepath.Join(tmp, ".config", "rtk", "config.toml")
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("no dest should be created when seed is absent")
	}
}

func TestEnsureRTKConfig_HonorsXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	xdg := filepath.Join(tmp, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)

	seedDir := filepath.Join(tmp, "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "rtk-config.toml"), []byte("[hooks]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.New(seedDir)
	if err := EnsureRTKConfig(cfg); err != nil {
		t.Fatalf("EnsureRTKConfig: %v", err)
	}

	dest := filepath.Join(xdg, "rtk", "config.toml")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("config should be written under XDG_CONFIG_HOME, stat %s: %v", dest, err)
	}
}
