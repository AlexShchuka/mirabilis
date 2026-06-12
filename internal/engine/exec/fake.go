package exec

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type FakeCall struct {
	Dir  string
	Argv []string
}

type fakeStub struct {
	err    error
	stdout string
	stderr string
	prefix []string
	code   int
	hang   bool
}

type Fake struct {
	mu    sync.Mutex
	stubs []fakeStub
	calls []FakeCall
}

var _ Runner = (*Fake)(nil)

func NewFake() *Fake { return &Fake{} }

func (f *Fake) Expect(argvPrefix []string, stdout string, err error) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	code := 0
	if err != nil {
		code = 1
	}
	f.stubs = append(f.stubs, fakeStub{prefix: argvPrefix, stdout: stdout, err: err, code: code})
	return f
}

func (f *Fake) ExpectHang(argvPrefix []string) *Fake {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stubs = append(f.stubs, fakeStub{prefix: argvPrefix, hang: true})
	return f
}

func (f *Fake) Calls() []FakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FakeCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *Fake) Remaining() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.stubs)
}

func (f *Fake) Stream(ctx context.Context, spec Spec) <-chan Event {
	f.mu.Lock()
	f.calls = append(f.calls, FakeCall{Argv: spec.Argv, Dir: spec.Dir})
	var stub fakeStub
	matched := false
	for i, s := range f.stubs {
		if hasPrefix(spec.Argv, s.prefix) {
			stub = s
			f.stubs = append(f.stubs[:i], f.stubs[i+1:]...)
			matched = true
			break
		}
	}
	f.mu.Unlock()

	ch := make(chan Event, 16)
	go func() {
		defer close(ch)
		ch <- Event{Kind: KindStarted, Argv: spec.Argv}
		if !matched {
			ch <- Event{Kind: KindExited, Code: 1, Err: fmt.Errorf("fake: unexpected call: %s", strings.Join(spec.Argv, " "))}
			return
		}
		if stub.hang {
			<-ctx.Done()
			ch <- Event{Kind: KindExited, Code: 1, Err: ctx.Err()}
			return
		}
		for _, line := range splitLines(stub.stdout) {
			ch <- Event{Kind: KindStdout, Line: line}
		}
		for _, line := range splitLines(stub.stderr) {
			ch <- Event{Kind: KindStderr, Line: line}
		}
		ch <- Event{Kind: KindExited, Code: stub.code, Err: stub.err}
	}()
	return ch
}

func hasPrefix(argv, prefix []string) bool {
	if len(prefix) > len(argv) {
		return false
	}
	for i, p := range prefix {
		if argv[i] != p {
			return false
		}
	}
	return true
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
