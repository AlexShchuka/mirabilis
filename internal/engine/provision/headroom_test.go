package provision

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func assertScriptCalls(t *testing.T, calls []exec.FakeCall, want []string) {
	t.Helper()
	if len(calls) != len(want) {
		t.Fatalf("calls = %d, want %d: %v", len(calls), len(want), calls)
	}
	for i, c := range calls {
		if len(c.Argv) != 3 || c.Argv[0] != "bash" || c.Argv[1] != "-lc" {
			t.Fatalf("call %d: argv = %v, want bash -lc script", i, c.Argv)
		}
		if c.Argv[2] != want[i] {
			t.Errorf("call %d script = %q, want %q", i, c.Argv[2], want[i])
		}
	}
}

func TestHeadroomCheckFalseWhenUpstreamDiffers(t *testing.T) {
	d, f := testDeps(t)
	d.ProxyAddr = "http://host.docker.internal:8788"
	mustWrite(t, d.upstreamPath(), "http://stale:9999\n")
	sc := headroomScripts(d)
	f.Expect(script(sc["probe"]), "", nil)
	step := &headroomStep{d: d}
	if checkStep(t, step) {
		t.Error("check should be false when the upstream file differs from ProxyAddr")
	}
	assertScriptCalls(t, f.Calls(), []string{sc["probe"]})
}

func TestHeadroomRunRestartsWhenUpstreamChangedWhileReachable(t *testing.T) {
	d, f := testDeps(t)
	d.ProxyAddr = "http://host.docker.internal:8788"
	mustWrite(t, d.upstreamPath(), "http://stale:9999\n")
	sc := headroomScripts(d)
	f.Expect(script(sc["probe"]), "", nil)
	f.Expect(script(sc["curl"]), "", nil)
	f.Expect(script(sc["pkill"]), "", nil)
	f.Expect(script(sc["curl"]), "", errors.New("down"))
	f.Expect(script(sc["start"]), "", nil)
	f.Expect(script(sc["poll"]), "", nil)
	f.Expect(script(sc["get"]), ".headroom-venv/bin/headroom", nil)
	step := &headroomStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(d.upstreamPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != d.ProxyAddr {
		t.Errorf("upstream file = %q, want %s", data, d.ProxyAddr)
	}
	assertScriptCalls(t, f.Calls(), []string{
		sc["probe"], sc["curl"], sc["pkill"], sc["curl"], sc["start"], sc["poll"], sc["get"],
	})
}

func TestHeadroomRunInstallsWhenMissing(t *testing.T) {
	d, f := testDeps(t)
	sc := headroomScripts(d)
	f.Expect(script(sc["probe"]), "", errors.New("missing"))
	f.Expect(script(sc["venv"]), "", nil)
	f.Expect(script(sc["pip"]), "", nil)
	f.Expect(script(sc["link"]), "", nil)
	f.Expect(script(sc["curl"]), "", nil)
	f.Expect(script(sc["curl"]), "", nil)
	f.Expect(script(sc["get"]), ".headroom-venv/bin/headroom", nil)
	step := &headroomStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exists(d.upstreamPath()) {
		t.Error("upstream file must not be written when ProxyAddr is empty")
	}
	assertScriptCalls(t, f.Calls(), []string{
		sc["probe"], sc["venv"], sc["pip"], sc["link"], sc["curl"], sc["curl"], sc["get"],
	})
}
