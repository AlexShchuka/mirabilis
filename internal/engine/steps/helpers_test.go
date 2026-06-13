package steps

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func drainStream(t *testing.T, events <-chan exec.Event) error {
	t.Helper()
	out := make(chan pipeline.Event, 256)
	err := stream("image", out, events)
	close(out)
	return err
}

func TestStreamWrapsExitErrorWithOutputTail(t *testing.T) {
	exitErr := errors.New("exit status 1")
	events := make(chan exec.Event, 4)
	events <- exec.Event{Kind: exec.KindStderr, Line: "failed to compute cache key"}
	events <- exec.Event{Kind: exec.KindStderr, Line: "\"/.build/mirabilis-linux\": not found"}
	events <- exec.Event{Kind: exec.KindExited, Err: exitErr}
	close(events)

	err := drainStream(t, events)
	if err == nil {
		t.Fatal("stream returned nil on failed exit")
	}
	if !errors.Is(err, exitErr) {
		t.Fatalf("wrapping broke the error chain: %q", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error did not carry the stderr tail: %q", err)
	}
}

func TestStreamSuccessReturnsNil(t *testing.T) {
	events := make(chan exec.Event, 2)
	events <- exec.Event{Kind: exec.KindStdout, Line: "Building"}
	events <- exec.Event{Kind: exec.KindExited, Err: nil}
	close(events)

	if err := drainStream(t, events); err != nil {
		t.Fatalf("stream on success = %v, want nil", err)
	}
}

func TestStreamTailIsBounded(t *testing.T) {
	events := make(chan exec.Event, streamTailLines+5)
	for i := 0; i < streamTailLines+3; i++ {
		events <- exec.Event{Kind: exec.KindStdout, Line: "line-" + strconv.Itoa(i)}
	}
	events <- exec.Event{Kind: exec.KindExited, Err: errors.New("boom")}
	close(events)

	err := drainStream(t, events)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "line-0") {
		t.Fatalf("tail kept a line beyond the bound: %q", err)
	}
	if !strings.Contains(err.Error(), "line-"+strconv.Itoa(streamTailLines+2)) {
		t.Fatalf("tail dropped the most recent line: %q", err)
	}
}
