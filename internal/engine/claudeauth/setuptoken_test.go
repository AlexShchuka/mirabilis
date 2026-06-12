package claudeauth

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/obs"
)

func newTestObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, err := obs.New(filepath.Join(t.TempDir(), "obs.log"))
	if err != nil {
		t.Fatalf("obs.New: %v", err)
	}
	t.Cleanup(func() { _ = o.Close() })
	return o
}

func writeAll(t *testing.T, e *Extractor, chunks ...string) {
	t.Helper()
	for _, c := range chunks {
		n, err := e.Write([]byte(c))
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
		if n != len(c) {
			t.Fatalf("Write = %d, want %d", n, len(c))
		}
	}
}

func TestSetupArgv(t *testing.T) {
	got := SetupArgv()
	want := []string{"claude", "setup-token"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("SetupArgv() = %v, want %v", got, want)
	}
}

func TestExtractorSingleChunk(t *testing.T) {
	e := NewExtractor()
	writeAll(t, e, "Your long-lived token:\r\n"+testToken+"\r\nDone.\r\n")

	got, ok := e.Token()
	if !ok || got != testToken {
		t.Fatalf("Token() = %q, %v, want %q, true", got, ok, testToken)
	}
}

func TestExtractorSplitAcrossChunks(t *testing.T) {
	e := NewExtractor()
	writeAll(t, e,
		"Token: sk-an",
		"t-oat01-"+testToken[13:50],
		testToken[50:]+"\r\n",
	)

	got, ok := e.Token()
	if !ok || got != testToken {
		t.Fatalf("Token() = %q, %v, want %q, true", got, ok, testToken)
	}
}

func TestExtractorANSIAroundTokenLine(t *testing.T) {
	e := NewExtractor()
	writeAll(t, e,
		"\x1b[2J\x1b[1;1H\x1b[1mSetup complete.\x1b[0m\r\n",
		testToken+"\r\n",
		"\x1b[?25h\x1b[0m",
	)

	got, ok := e.Token()
	if !ok || got != testToken {
		t.Fatalf("Token() = %q, %v, want %q, true", got, ok, testToken)
	}
}

func TestExtractorANSIInsideTokenBreaksMatch(t *testing.T) {
	e := NewExtractor()
	writeAll(t, e, "sk-ant-", "\x1b[K", "oat01-abcdef0123456789\r\n")

	if got, ok := e.Token(); ok {
		t.Fatalf("Token() = %q, true, want false on ANSI-split raw stream", got)
	}
}

func TestExtractorLastMatchWins(t *testing.T) {
	first := tokenPrefix + strings.Repeat("a", 95)
	second := tokenPrefix + strings.Repeat("b", 95)
	e := NewExtractor()
	writeAll(t, e,
		"old: "+first+"\r\n",
		"some output in between\r\n",
		"new: "+second+"\r\n",
	)

	got, ok := e.Token()
	if !ok || got != second {
		t.Fatalf("Token() = %q, %v, want %q, true", got, ok, second)
	}
}

func TestExtractorNoMatch(t *testing.T) {
	e := NewExtractor()
	if got, ok := e.Token(); ok {
		t.Fatalf("Token() on empty stream = %q, true, want false", got)
	}
	writeAll(t, e, "no token in this output\r\nsk-ant-api03-other\r\n")

	if got, ok := e.Token(); ok {
		t.Fatalf("Token() = %q, true, want false", got)
	}
}

func TestExtractorPassThrough(t *testing.T) {
	var tee bytes.Buffer
	e := NewExtractor()
	e.Tee(&tee)
	chunks := []string{
		"\x1b[2Jbanner\r\n",
		"Token: sk-an",
		"t-oat01-" + testToken[13:] + "\r\n",
		"\x00\xff binary-ish tail",
	}
	writeAll(t, e, chunks...)

	if got, want := tee.String(), strings.Join(chunks, ""); got != want {
		t.Fatalf("tee output = %q, want %q", got, want)
	}
	got, ok := e.Token()
	if !ok || got != testToken {
		t.Fatalf("Token() = %q, %v, want %q, true", got, ok, testToken)
	}
}

func TestExtractorSlidingWindowKeepsBoundaryToken(t *testing.T) {
	e := NewExtractor()
	filler := strings.Repeat("x", 1000) + "\r\n"
	for range 9 {
		writeAll(t, e, filler)
	}
	writeAll(t, e, "Token: sk-ant-oat", "01-"+testToken[13:]+"\r\n")
	writeAll(t, e, filler, filler, filler, filler, filler)

	got, ok := e.Token()
	if !ok || got != testToken {
		t.Fatalf("Token() = %q, %v, want %q, true", got, ok, testToken)
	}
}

func waitForNode(t *testing.T, o *obs.Obs, want obs.State) obs.NodeStatus {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if st, ok := o.Snapshot()[obsNode]; ok {
			if st.State != want {
				t.Fatalf("obs %q state = %v, want %v", obsNode, st.State, want)
			}
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("obs %q never set", obsNode)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestStoreInBackgroundSuccess(t *testing.T) {
	store := newFakeStore()
	o := newTestObs(t)

	StoreInBackground(store, testToken, o)

	st := waitForNode(t, o, obs.StateOK)
	if strings.Contains(st.Detail, testToken) {
		t.Fatalf("obs detail %q leaks the token", st.Detail)
	}
	if got, ok := store.value(tokenKey); !ok || got != testToken {
		t.Fatalf("store value = %q, %v, want %q, true", got, ok, testToken)
	}
}

func TestStoreInBackgroundFailure(t *testing.T) {
	store := newFakeStore()
	store.setErr = errors.New("keychain locked")
	o := newTestObs(t)

	StoreInBackground(store, testToken, o)

	st := waitForNode(t, o, obs.StateDegraded)
	if strings.Contains(st.Detail, testToken) {
		t.Fatalf("obs detail %q leaks the token", st.Detail)
	}
}
