package claudeauth

import (
	"context"
	"errors"
	"testing"
)

type tokenSourceFunc func(ctx context.Context) (string, error)

func (f tokenSourceFunc) Token(ctx context.Context) (string, error) {
	return f(ctx)
}

func TestPresent(t *testing.T) {
	tests := []struct {
		err   error
		name  string
		token string
		want  bool
	}{
		{name: "valid oat token", token: testToken, want: true},
		{name: "error", err: errors.New("not found"), want: false},
		{name: "non-oat token", token: "sk-ant-api03-something", want: false},
		{name: "empty", token: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := tokenSourceFunc(func(context.Context) (string, error) {
				return tt.token, tt.err
			})
			if got := Present(context.Background(), ts); got != tt.want {
				t.Errorf("Present() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPresentWithCachedSource(t *testing.T) {
	store := newFakeStore()
	src := NewSource(store)
	ctx := context.Background()

	if Present(ctx, src) {
		t.Fatal("Present() = true on empty store, want false")
	}

	store.put(tokenKey, testToken)
	if !Present(ctx, src) {
		t.Fatal("Present() = false after store gains token, want true")
	}
}
