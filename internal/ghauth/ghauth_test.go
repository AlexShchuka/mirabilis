package ghauth

import (
	"context"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestParseUserCode(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{give: "! First copy your one-time code: ABCD-1234", want: "ABCD-1234"},
		{give: "code: 2741-EE59 now", want: "2741-EE59"},
		{give: "no code here", want: ""},
		{give: "lowercase abcd-1234 is ignored", want: ""},
	}
	for _, tt := range tests {
		if got := ParseUserCode(tt.give); got != tt.want {
			t.Errorf("ParseUserCode(%q) = %q, want %q", tt.give, got, tt.want)
		}
	}
}

func TestParseDeviceURL(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{give: "Open https://github.com/login/device to continue", want: "https://github.com/login/device"},
		{give: "go to https://github.com/login/device.", want: "https://github.com/login/device"},
		{give: "no url", want: ""},
	}
	for _, tt := range tests {
		if got := ParseDeviceURL(tt.give); got != tt.want {
			t.Errorf("ParseDeviceURL(%q) = %q, want %q", tt.give, got, tt.want)
		}
	}
}

func TestLoginArgsRequestWorkflowScope(t *testing.T) {
	args := loginArgs()
	has := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}
	if !has("gh") || !has("auth") || !has("login") {
		t.Fatalf("loginArgs is not a gh auth login invocation: %v", args)
	}
	scoped := false
	for i, a := range args {
		if a == "--scopes" && i+1 < len(args) && args[i+1] == "workflow" {
			scoped = true
		}
	}
	if !scoped {
		t.Errorf("loginArgs must request the workflow scope, got %v", args)
	}
}

func TestRunExitsCleanlyOnStartError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g := New(ctx, &runner.FakeRunner{}, 80, 24)
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)

	done := make(chan struct{})
	go func() {
		defer close(done)
		g.run()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("run() did not return after process start failure")
	}
}

func TestOnLineCapturesAndOpensOnce(t *testing.T) {
	g := New(context.Background(), &runner.FakeRunner{}, 80, 24)

	g.onLine("! First copy your one-time code: ABCD-1234")
	if g.code != "ABCD-1234" {
		t.Errorf("code = %q, want ABCD-1234", g.code)
	}
	if g.opened {
		t.Error("must not open the browser before the URL is known")
	}

	g.onLine("Open https://github.com/login/device")
	if g.url != "https://github.com/login/device" {
		t.Errorf("url = %q, want the device URL", g.url)
	}
	if !g.opened {
		t.Error("should open the browser once both code and URL are known")
	}
}
