package runtime

import (
	"bytes"
	"testing"
)

func TestResetTerminal(t *testing.T) {
	var buf bytes.Buffer
	resetTerminal(&buf)
	want := exitAltScreen + resetScrollRegion + showCursor
	got := buf.String()
	if got == "" {
		t.Fatal("resetTerminal wrote nothing")
	}
	if got != want {
		t.Fatalf("resetTerminal output mismatch: got %q, want %q", got, want)
	}
}
