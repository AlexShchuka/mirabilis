package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func findNewArgv(acct, key string) []string {
	return []string{"security", "find-generic-password", "-a", acct, "-s", "mirabilis-" + key, "-w"}
}

func findOldArgv(acct, key string) []string {
	return []string{"security", "find-generic-password", "-a", acct, "-s", "mirabilis-" + key + "-token", "-w"}
}

func addNewArgv(acct, key string) []string {
	return []string{"security", "add-generic-password", "-U", "-a", acct, "-s", "mirabilis-" + key, "-w"}
}

func deleteOldArgv(acct, key string) []string {
	return []string{"security", "delete-generic-password", "-a", acct, "-s", "mirabilis-" + key + "-token"}
}

func TestKeychainStoreGet(t *testing.T) {
	const (
		acct = "tester"
		key  = "claude-token"
	)
	exitErr := errors.New("exit status 44")

	tests := []struct {
		wantErr      error
		stub         func(f *exec.Fake)
		name         string
		legacyFile   string
		want         string
		wantCalls    [][]string
		wantFileGone bool
	}{
		{
			name: "new name hit no extra calls",
			stub: func(f *exec.Fake) {
				f.Expect(findNewArgv(acct, key), "tok-new\n", nil)
			},
			want:      "tok-new",
			wantCalls: [][]string{findNewArgv(acct, key)},
		},
		{
			name:       "migrates legacy keychain entry and removes legacy file",
			legacyFile: "stale-plaintext-copy\n",
			stub: func(f *exec.Fake) {
				f.Expect(findNewArgv(acct, key), "", exitErr)
				f.Expect(findOldArgv(acct, key), "tok-old\n", nil)
				f.Expect(addNewArgv(acct, key), "", nil)
				f.Expect(deleteOldArgv(acct, key), "", nil)
			},
			want: "tok-old",
			wantCalls: [][]string{
				findNewArgv(acct, key),
				findOldArgv(acct, key),
				addNewArgv(acct, key),
				deleteOldArgv(acct, key),
			},
			wantFileGone: true,
		},
		{
			name:       "migrates legacy plaintext file into keychain",
			legacyFile: "tok-file\n",
			stub: func(f *exec.Fake) {
				f.Expect(findNewArgv(acct, key), "", exitErr)
				f.Expect(findOldArgv(acct, key), "", exitErr)
				f.Expect(addNewArgv(acct, key), "", nil)
			},
			want: "tok-file",
			wantCalls: [][]string{
				findNewArgv(acct, key),
				findOldArgv(acct, key),
				addNewArgv(acct, key),
			},
			wantFileGone: true,
		},
		{
			name: "all miss",
			stub: func(f *exec.Fake) {
				f.Expect(findNewArgv(acct, key), "", exitErr)
				f.Expect(findOldArgv(acct, key), "", exitErr)
			},
			wantErr: ErrNotFound,
			wantCalls: [][]string{
				findNewArgv(acct, key),
				findOldArgv(acct, key),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", acct)
			dir := t.TempDir()
			legacyPath := filepath.Join(dir, ".mirabilis-"+key)
			if tt.legacyFile != "" {
				if err := os.WriteFile(legacyPath, []byte(tt.legacyFile), 0o600); err != nil {
					t.Fatalf("seed legacy file: %v", err)
				}
			}
			fake := exec.NewFake()
			tt.stub(fake)
			store := NewKeychainStore(fake, dir)

			got, err := store.Get(context.Background(), key)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Get err = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got != tt.want {
				t.Errorf("Get = %q, want %q", got, tt.want)
			}

			calls := fake.Calls()
			if len(calls) != len(tt.wantCalls) {
				t.Fatalf("calls = %d, want %d: %+v", len(calls), len(tt.wantCalls), calls)
			}
			for i := range calls {
				if !slices.Equal(calls[i].Argv, tt.wantCalls[i]) {
					t.Errorf("call %d argv = %v, want %v", i, calls[i].Argv, tt.wantCalls[i])
				}
			}

			if tt.wantFileGone {
				if _, err := os.Stat(legacyPath); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("legacy file still present, stat err = %v", err)
				}
			}
		})
	}
}

func TestKeychainStoreSetValueNotInArgv(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "tester")
	const secret = "s3cr3t-value"
	fake := exec.NewFake()
	fake.Expect([]string{"security", "add-generic-password"}, "", nil)
	store := NewKeychainStore(fake, t.TempDir())

	if err := store.Set(context.Background(), "telegram-token", secret); err != nil {
		t.Fatalf("Set: %v", err)
	}

	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	argv := calls[0].Argv
	if !slices.Equal(argv, addNewArgv("tester", "telegram-token")) {
		t.Errorf("argv = %v, want %v", argv, addNewArgv("tester", "telegram-token"))
	}
	if argv[len(argv)-1] != "-w" {
		t.Errorf("last argv = %q, want -w", argv[len(argv)-1])
	}
	for _, call := range calls {
		for _, a := range call.Argv {
			if strings.Contains(a, secret) {
				t.Errorf("secret value leaked into argv: %v", call.Argv)
			}
		}
	}
}

func TestKeychainStoreGetTimeout(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "tester")
	fake := exec.NewFake()
	fake.ExpectHang([]string{"security", "find-generic-password"})
	store := NewKeychainStore(fake, t.TempDir())
	store.timeout = 50 * time.Millisecond

	start := time.Now()
	_, err := store.Get(context.Background(), "claude-token")
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("Get took %v, want under 2s", elapsed)
	}
	if err == nil {
		t.Fatal("Get err = nil, want deadline error")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("Get err = ErrNotFound, want ctx error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Get err = %v, want context.DeadlineExceeded", err)
	}
}

func TestKeychainAccountResolution(t *testing.T) {
	tests := []struct {
		name    string
		envAcct string
		envUser string
		want    string
	}{
		{name: "explicit override", envAcct: "override-acct", envUser: "someone", want: "override-acct"},
		{name: "user fallback", envUser: "someone", want: "someone"},
		{name: "default account", want: "mirabilis"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", tt.envAcct)
			t.Setenv("USER", tt.envUser)
			fake := exec.NewFake()
			fake.Expect(findNewArgv(tt.want, "claude-token"), "tok\n", nil)
			store := NewKeychainStore(fake, t.TempDir())

			got, err := store.Get(context.Background(), "claude-token")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got != "tok" {
				t.Errorf("Get = %q, want %q", got, "tok")
			}
			calls := fake.Calls()
			if len(calls) != 1 {
				t.Fatalf("calls = %d, want 1: %+v", len(calls), calls)
			}
			if !slices.Equal(calls[0].Argv, findNewArgv(tt.want, "claude-token")) {
				t.Errorf("argv = %v, want account %q", calls[0].Argv, tt.want)
			}
		})
	}
}
