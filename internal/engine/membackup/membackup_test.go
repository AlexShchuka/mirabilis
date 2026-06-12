package membackup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func TestSaveArgv(t *testing.T) {
	repo := t.TempDir()
	f := exec.NewFake().Expect([]string{"docker", "cp"}, "", nil)

	if err := Save(context.Background(), f, repo); err != nil {
		t.Fatalf("Save: %v", err)
	}
	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	want := []string{
		"docker", "cp",
		"mirabilis:/home/node/.claude/memory",
		filepath.Join(repo, ".mirabilis", "saved-memory"),
	}
	if !slices.Equal(calls[0].Argv, want) {
		t.Fatalf("argv = %v, want %v", calls[0].Argv, want)
	}
}

func TestSaveClearsPreviousSnapshot(t *testing.T) {
	repo := t.TempDir()
	dst := filepath.Join(repo, ".mirabilis", "saved-memory")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dst, "stale.md"), []byte("old"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	f := exec.NewFake().Expect([]string{"docker", "cp"}, "", nil)

	if err := Save(context.Background(), f, repo); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale snapshot still present: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dst)); err != nil {
		t.Fatalf("parent dir missing: %v", err)
	}
}

func TestSaveError(t *testing.T) {
	repo := t.TempDir()
	errCp := errors.New("no such container")
	f := exec.NewFake().Expect([]string{"docker", "cp"}, "", errCp)

	err := Save(context.Background(), f, repo)
	if !errors.Is(err, errCp) {
		t.Fatalf("Save error = %v, want wrapping %v", err, errCp)
	}
}
