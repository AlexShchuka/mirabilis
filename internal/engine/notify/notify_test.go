package notify

import (
	"context"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/secrets"
)

func TestRegistryFakeAdapter(t *testing.T) {
	want := &fakeNotifier{}
	Register("fake-demo", func(secrets.Store) (Notifier, error) {
		return want, nil
	})

	got, err := New("fake-demo", secrets.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatalf("New(fake-demo): %v", err)
	}
	if got != Notifier(want) {
		t.Errorf("New returned %T, want the registered fake instance", got)
	}
}

func TestRegistryUnknownName(t *testing.T) {
	_, err := New("no-such-adapter", secrets.NewFileStore(t.TempDir()))
	if err == nil {
		t.Fatal("New(no-such-adapter) = nil, want error")
	}
	if !strings.Contains(err.Error(), "no-such-adapter") {
		t.Errorf("error = %q, want it to name the adapter", err)
	}
}

func TestRegistryTelegramRegistered(t *testing.T) {
	n, err := New("telegram", secrets.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatalf("New(telegram): %v", err)
	}
	if _, ok := n.(*Telegram); !ok {
		t.Errorf("New(telegram) = %T, want *Telegram", n)
	}
}

func TestRegistryConstructorReceivesStore(t *testing.T) {
	store := secrets.NewFileStore(t.TempDir())
	if err := store.Set(context.Background(), TokenKey, "tok-reg"); err != nil {
		t.Fatal(err)
	}
	var gotStore secrets.Store
	Register("store-probe", func(s secrets.Store) (Notifier, error) {
		gotStore = s
		return &fakeNotifier{}, nil
	})
	if _, err := New("store-probe", store); err != nil {
		t.Fatalf("New(store-probe): %v", err)
	}
	if gotStore != secrets.Store(store) {
		t.Error("constructor did not receive the caller's store")
	}
}
