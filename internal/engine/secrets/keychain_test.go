package secrets

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/user"
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

func setArgv() []string {
	return []string{"security", "-i"}
}

func deleteOldArgv(acct, key string) []string {
	return []string{"security", "delete-generic-password", "-a", acct, "-s", "mirabilis-" + key + "-token"}
}

func b64Stored(v string) string {
	return b64Prefix + base64.StdEncoding.EncodeToString([]byte(v))
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
				f.Expect(findNewArgv(acct, key), b64Stored("tok-new")+"\n", nil)
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
				f.Expect(setArgv(), "", nil)
				f.Expect(findNewArgv(acct, key), b64Stored("tok-old")+"\n", nil)
				f.Expect(deleteOldArgv(acct, key), "", nil)
			},
			want: "tok-old",
			wantCalls: [][]string{
				findNewArgv(acct, key),
				findOldArgv(acct, key),
				setArgv(),
				findNewArgv(acct, key),
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
				f.Expect(setArgv(), "", nil)
				f.Expect(findNewArgv(acct, key), b64Stored("tok-file")+"\n", nil)
			},
			want: "tok-file",
			wantCalls: [][]string{
				findNewArgv(acct, key),
				findOldArgv(acct, key),
				setArgv(),
				findNewArgv(acct, key),
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

func TestKeychainStoreSetUsesSecurityInteractive(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "tester")
	const secret = "s3cr3t-value"
	fake := exec.NewFake()
	fake.Expect(setArgv(), "", nil)
	fake.Expect(findNewArgv("tester", "telegram-token"), b64Stored(secret)+"\n", nil)
	store := NewKeychainStore(fake, t.TempDir())

	if err := store.Set(context.Background(), "telegram-token", secret); err != nil {
		t.Fatalf("Set: %v", err)
	}

	calls := fake.Calls()
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2 (set + readback)", len(calls))
	}
	if !slices.Equal(calls[0].Argv, setArgv()) {
		t.Errorf("set argv = %v, want %v", calls[0].Argv, setArgv())
	}
	for _, a := range calls[0].Argv {
		if strings.Contains(a, secret) {
			t.Errorf("secret value leaked into argv: %v", calls[0].Argv)
		}
	}
	stdinBytes, err := io.ReadAll(calls[0].Stdin)
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	stdin := string(stdinBytes)
	if !strings.Contains(stdin, "add-generic-password") {
		t.Errorf("stdin missing add-generic-password command: %q", stdin)
	}
	if strings.Contains(stdin, secret) {
		t.Errorf("raw secret visible in stdin command (should be base64): %q", stdin)
	}
	if !strings.Contains(stdin, b64Prefix) {
		t.Errorf("stdin missing b64 prefix %q: %q", b64Prefix, stdin)
	}
}

func TestKeychainStoreSetVerifiesRoundtrip(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "tester")
	const secret = "my-secret"

	t.Run("readback matches returns nil", func(t *testing.T) {
		fake := exec.NewFake()
		fake.Expect(setArgv(), "", nil)
		fake.Expect(findNewArgv("tester", "test-key"), b64Stored(secret)+"\n", nil)
		store := NewKeychainStore(fake, t.TempDir())
		if err := store.Set(context.Background(), "test-key", secret); err != nil {
			t.Fatalf("Set: %v", err)
		}
	})

	t.Run("readback empty returns error", func(t *testing.T) {
		fake := exec.NewFake()
		fake.Expect(setArgv(), "", nil)
		fake.Expect(findNewArgv("tester", "test-key"), "\n", nil)
		store := NewKeychainStore(fake, t.TempDir())
		if err := store.Set(context.Background(), "test-key", secret); err == nil {
			t.Fatal("Set returned nil on empty readback, want error")
		}
	})

	t.Run("readback mismatch returns error", func(t *testing.T) {
		fake := exec.NewFake()
		fake.Expect(setArgv(), "", nil)
		fake.Expect(findNewArgv("tester", "test-key"), b64Stored("different-value")+"\n", nil)
		store := NewKeychainStore(fake, t.TempDir())
		if err := store.Set(context.Background(), "test-key", secret); err == nil {
			t.Fatal("Set returned nil on mismatch readback, want error")
		}
	})
}

func TestKeychainStoreGetDecodesB64(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "tester")
	const want = "decoded-secret"
	fake := exec.NewFake()
	fake.Expect(findNewArgv("tester", "mykey"), b64Stored(want)+"\n", nil)
	store := NewKeychainStore(fake, t.TempDir())
	got, err := store.Get(context.Background(), "mykey")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != want {
		t.Errorf("Get = %q, want %q", got, want)
	}
}

func TestKeychainStoreGetRawLegacyPassthrough(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "tester")
	const rawValue = "raw-legacy-token"
	fake := exec.NewFake()
	fake.Expect(findNewArgv("tester", "mykey"), rawValue+"\n", nil)
	store := NewKeychainStore(fake, t.TempDir())
	got, err := store.Get(context.Background(), "mykey")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != rawValue {
		t.Errorf("Get = %q, want %q", got, rawValue)
	}
}

func TestKeychainStoreMigrateNoDeleteOnFailedSet(t *testing.T) {
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", "tester")
	const key = "claude-token"
	const acct = "tester"
	exitErr := errors.New("exit status 44")

	fake := exec.NewFake()
	fake.Expect(findNewArgv(acct, key), "", exitErr)
	fake.Expect(findOldArgv(acct, key), "tok-old\n", nil)
	fake.Expect(setArgv(), "", nil)
	fake.Expect(findNewArgv(acct, key), "\n", nil)

	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".mirabilis-"+key)
	if err := os.WriteFile(legacyPath, []byte("tok-old\n"), 0o600); err != nil {
		t.Fatalf("seed legacy file: %v", err)
	}

	store := NewKeychainStore(fake, dir)
	_, err := store.Get(context.Background(), key)
	if err == nil {
		t.Fatal("Get returned nil on failed migration Set, want error")
	}

	if _, serr := os.Stat(legacyPath); errors.Is(serr, os.ErrNotExist) {
		t.Error("legacy file was deleted after failed Set, want it preserved")
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
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}

	tests := []struct {
		name    string
		envAcct string
		want    string
	}{
		{name: "explicit override", envAcct: "override-acct", want: "override-acct"},
		{name: "os user fallback", want: currentUser.Username},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", tt.envAcct)
			fake := exec.NewFake()
			fake.Expect(findNewArgv(tt.want, "claude-token"), b64Stored("tok")+"\n", nil)
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
