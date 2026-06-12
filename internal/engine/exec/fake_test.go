package exec

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func drain(ch <-chan Event) []Event {
	var evs []Event
	for ev := range ch {
		evs = append(evs, ev)
	}
	return evs
}

func equalLines(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestFakeOrderedMatching(t *testing.T) {
	f := NewFake().
		Expect([]string{"echo"}, "a", nil).
		Expect([]string{"echo"}, "b", nil)

	out1, err1 := Run(context.Background(), f, Spec{Argv: []string{"echo", "x"}})
	if err1 != nil || out1 != "a" {
		t.Fatalf("first call = (%q, %v), want (a, nil)", out1, err1)
	}
	out2, err2 := Run(context.Background(), f, Spec{Argv: []string{"echo", "y"}})
	if err2 != nil || out2 != "b" {
		t.Fatalf("second call = (%q, %v), want (b, nil)", out2, err2)
	}
	if f.Remaining() != 0 {
		t.Fatalf("remaining = %d, want 0", f.Remaining())
	}
}

func TestFakePrefixMatch(t *testing.T) {
	f := NewFake().Expect([]string{"git", "status"}, "clean", nil)
	out, err := Run(context.Background(), f, Spec{Argv: []string{"git", "status", "--short"}})
	if err != nil || out != "clean" {
		t.Fatalf("call = (%q, %v), want (clean, nil)", out, err)
	}
}

func TestFakeUnexpectedCall(t *testing.T) {
	f := NewFake()
	evs := drain(f.Stream(context.Background(), Spec{Argv: []string{"nope"}}))
	if len(evs) != 2 {
		t.Fatalf("events = %d, want 2", len(evs))
	}
	if evs[0].Kind != KindStarted {
		t.Fatalf("first event = %v, want KindStarted", evs[0].Kind)
	}
	if evs[1].Kind != KindExited {
		t.Fatalf("second event = %v, want KindExited", evs[1].Kind)
	}
	if evs[1].Err == nil || !strings.Contains(evs[1].Err.Error(), "unexpected call") {
		t.Fatalf("exit err = %v, want it to contain %q", evs[1].Err, "unexpected call")
	}
}

func TestFakeExpectHangHonorsCtx(t *testing.T) {
	f := NewFake().ExpectHang([]string{"sleep"})
	ctx, cancel := context.WithCancel(context.Background())
	ch := f.Stream(ctx, Spec{Argv: []string{"sleep", "30"}})

	if ev := <-ch; ev.Kind != KindStarted {
		t.Fatalf("first event = %v, want KindStarted", ev.Kind)
	}
	cancel()
	ev := <-ch
	if ev.Kind != KindExited {
		t.Fatalf("event = %v, want KindExited", ev.Kind)
	}
	if !errors.Is(ev.Err, context.Canceled) {
		t.Fatalf("exit err = %v, want context.Canceled", ev.Err)
	}
	if _, ok := <-ch; ok {
		t.Fatal("channel not closed after KindExited")
	}
}

func TestFakeCallsRecording(t *testing.T) {
	f := NewFake().Expect([]string{"a"}, "", nil)
	drain(f.Stream(context.Background(), Spec{Argv: []string{"a", "1"}, Dir: "/tmp"}))

	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if !equalLines(calls[0].Argv, []string{"a", "1"}) {
		t.Fatalf("call argv = %v, want [a 1]", calls[0].Argv)
	}
	if calls[0].Dir != "/tmp" {
		t.Fatalf("call dir = %q, want /tmp", calls[0].Dir)
	}
}

func TestFakeRemaining(t *testing.T) {
	f := NewFake().
		Expect([]string{"a"}, "", nil).
		Expect([]string{"b"}, "", nil)
	if f.Remaining() != 2 {
		t.Fatalf("remaining = %d, want 2", f.Remaining())
	}
	drain(f.Stream(context.Background(), Spec{Argv: []string{"a"}}))
	if f.Remaining() != 1 {
		t.Fatalf("remaining = %d, want 1", f.Remaining())
	}
}
