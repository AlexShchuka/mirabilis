//go:build darwin && integration

package secrets

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func deleteKeychainEntry(t *testing.T, acct, service string) {
	t.Helper()
	_, _ = exec.Run(context.Background(), exec.NewHost(), exec.Spec{
		Argv: []string{"security", "delete-generic-password", "-a", acct, "-s", service},
	})
}

func TestKeychainIntegrationRoundtrip(t *testing.T) {
	acct := fmt.Sprintf("mirabilis-ci-%d", os.Getpid())
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", acct)

	service := "ci-roundtrip"
	t.Cleanup(func() { deleteKeychainEntry(t, acct, "mirabilis-"+service) })

	store := NewKeychainStore(exec.NewHost(), t.TempDir())
	ctx := context.Background()

	const secret = "integration-test-secret-value"
	if err := store.Set(ctx, service, secret); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := store.Get(ctx, service)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != secret {
		t.Errorf("Get = %q, want %q", got, secret)
	}

	const secret2 = "integration-test-overwrite-value"
	if err := store.Set(ctx, service, secret2); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	got2, err := store.Get(ctx, service)
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if got2 != secret2 {
		t.Errorf("Get after overwrite = %q, want %q", got2, secret2)
	}
}

func TestKeychainIntegrationRawLegacyCompat(t *testing.T) {
	acct := fmt.Sprintf("mirabilis-ci-%d", os.Getpid())
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", acct)

	service := "mirabilis-ci-raw"
	t.Cleanup(func() { deleteKeychainEntry(t, acct, service) })

	ctx := context.Background()
	_, err := exec.Run(ctx, exec.NewHost(), exec.Spec{
		Argv: []string{"security", "add-generic-password", "-U", "-a", acct, "-s", service, "-w", "rawvalue123"},
	})
	if err != nil {
		t.Fatalf("seed raw entry: %v", err)
	}

	store := NewKeychainStore(exec.NewHost(), t.TempDir())

	got, err := store.Get(ctx, "ci-raw")
	if err != nil {
		t.Fatalf("Get raw legacy: %v", err)
	}
	if got != "rawvalue123" {
		t.Errorf("Get raw legacy = %q, want %q", got, "rawvalue123")
	}
}

func TestKeychainIntegrationCleanup(t *testing.T) {
	acct := fmt.Sprintf("mirabilis-ci-%d", os.Getpid())
	t.Setenv("MIRABILIS_KEYCHAIN_ACCOUNT", acct)

	service := "ci-cleanup-check"
	t.Cleanup(func() { deleteKeychainEntry(t, acct, "mirabilis-"+service) })

	store := NewKeychainStore(exec.NewHost(), t.TempDir())
	ctx := context.Background()

	if err := store.Set(ctx, service, "temporary"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	deleteKeychainEntry(t, acct, "mirabilis-"+service)

	_, err := exec.Run(ctx, exec.NewHost(), exec.Spec{
		Argv: []string{"security", "find-generic-password", "-a", acct, "-s", "mirabilis-" + service},
	})
	if err == nil {
		t.Error("entry still present after cleanup, want it deleted")
	}
}
