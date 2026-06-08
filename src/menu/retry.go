package main

import (
	"context"
	"math/rand"
	"time"
)

type RetryPolicy struct {
	Attempts  int
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

var (
	retryNet  = RetryPolicy{Attempts: 4, BaseDelay: 300 * time.Millisecond, MaxDelay: 8 * time.Second}
	retryNone = RetryPolicy{Attempts: 1}
)

func (p RetryPolicy) delay(attempt int) time.Duration {
	if p.BaseDelay <= 0 {
		return 0
	}
	d := p.BaseDelay << attempt
	if p.MaxDelay > 0 && d > p.MaxDelay {
		d = p.MaxDelay
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}

func retry(ctx context.Context, p RetryPolicy, fn func() error) error {
	attempts := p.Attempts
	if attempts < 1 {
		attempts = 1
	}
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i == attempts-1 {
			break
		}
		select {
		case <-time.After(p.delay(i)):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
