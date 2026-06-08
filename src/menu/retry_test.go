package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetrySucceedsFirstTry(t *testing.T) {
	calls := 0
	err := retry(context.Background(), RetryPolicy{Attempts: 4}, func() error {
		calls++
		return nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v, want 1 call and nil", calls, err)
	}
}

func TestRetryEventuallySucceeds(t *testing.T) {
	calls := 0
	err := retry(context.Background(), RetryPolicy{Attempts: 4}, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("calls=%d err=%v, want 3 calls and nil", calls, err)
	}
}

func TestRetryExhausts(t *testing.T) {
	calls := 0
	want := errors.New("boom")
	err := retry(context.Background(), RetryPolicy{Attempts: 3}, func() error {
		calls++
		return want
	})
	if calls != 3 || !errors.Is(err, want) {
		t.Fatalf("calls=%d err=%v, want 3 calls and boom", calls, err)
	}
}

func TestRetryAttemptsFloor(t *testing.T) {
	calls := 0
	_ = retry(context.Background(), RetryPolicy{Attempts: 0}, func() error {
		calls++
		return errors.New("x")
	})
	if calls != 1 {
		t.Fatalf("calls=%d, want 1 (attempts floored to 1)", calls)
	}
}

func TestRetryContextCancelStopsWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := retry(ctx, RetryPolicy{Attempts: 5, BaseDelay: time.Hour}, func() error {
		calls++
		return errors.New("x")
	})
	if calls != 1 || err == nil {
		t.Fatalf("calls=%d err=%v, want 1 call and a context error", calls, err)
	}
}

func TestDelayWithinJitterBounds(t *testing.T) {
	p := RetryPolicy{BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	for attempt := 0; attempt < 6; attempt++ {
		base := p.BaseDelay << attempt
		if base > p.MaxDelay {
			base = p.MaxDelay
		}
		for i := 0; i < 50; i++ {
			d := p.delay(attempt)
			if d < base/2 || d > base {
				t.Fatalf("attempt %d: delay %v out of [%v,%v]", attempt, d, base/2, base)
			}
		}
	}
}

func TestDelayZeroBaseIsZero(t *testing.T) {
	if d := (RetryPolicy{}).delay(3); d != 0 {
		t.Errorf("zero base delay = %v, want 0", d)
	}
}
