//go:build darwin || linux

package main

import (
	"runtime"
	"testing"
)

func TestFlockSurvivesGC(t *testing.T) {
	dir := t.TempDir()

	if err := acquireFlock(dir); err != nil {
		t.Fatalf("first acquireFlock: %v", err)
	}

	runtime.GC()
	runtime.GC()

	if err := acquireFlock(dir); err == nil {
		t.Fatal("second acquireFlock succeeded after GC — lock was released (GC bug)")
	}
}

func TestFlockReleaseAllowsReacquire(t *testing.T) {
	dir := t.TempDir()
	if err := acquireFlock(dir); err != nil {
		t.Fatalf("first acquireFlock: %v", err)
	}
	releaseFlock()
	if err := acquireFlock(dir); err != nil {
		t.Fatalf("acquireFlock after release: %v", err)
	}
	releaseFlock()
}
