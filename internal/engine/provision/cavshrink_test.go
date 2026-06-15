package provision

import (
	"path/filepath"
	"strings"
	"testing"
)

func writeShrinkMCPCatalog(t *testing.T, repo string) {
	t.Helper()
	mustWrite(t, filepath.Join(repo, "config", "mcp.json"),
		`[{"name":"seq","transport":"stdio","args":["npx","-y","@mcp/seq"],"shrink":true},`+
			`{"name":"ctx","transport":"http","url":"https://mcp.ctx.com","shrink":false}]`)
}

func TestCaveShrinkCheckTrueWhenNoTargets(t *testing.T) {
	d, _ := testDeps(t)
	mustWrite(t, filepath.Join(d.Repo, "config", "mcp.json"),
		`[{"name":"ctx","transport":"http","url":"https://mcp.ctx.com"}]`)
	step := &caveShrinkStep{d: d}
	ok, err := step.Check(t.Context())
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if !ok {
		t.Error("Check = false with no shrink targets, want true")
	}
}

func TestCaveShrinkRunCallsNpxForShrinkTargets(t *testing.T) {
	d, f := testDeps(t)
	writeShrinkMCPCatalog(t, d.Repo)
	f.Expect([]string{"npx", "caveman-shrink", "--with-mcp-shrink=npx -y @mcp/seq"}, "", nil)
	step := &caveShrinkStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("%d expected calls not made", n)
	}
}

func TestCaveShrinkRunCheckIdempotent(t *testing.T) {
	d, f := testDeps(t)
	writeShrinkMCPCatalog(t, d.Repo)
	f.Expect([]string{"npx", "caveman-shrink", "--with-mcp-shrink=npx -y @mcp/seq"}, "", nil)
	step := &caveShrinkStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("first Run error: %v", err)
	}
	if ok, err := step.Check(t.Context()); err != nil {
		t.Fatalf("Check error: %v", err)
	} else if !ok {
		t.Error("Check = false after Run (INV-ORGAN-U idempotency)")
	}
	if n := f.Remaining(); n != 0 {
		t.Errorf("%d stubs unused after idempotency check", n)
	}
}

func TestCaveShrinkRunSkipsHTTPTransport(t *testing.T) {
	d, f := testDeps(t)
	writeShrinkMCPCatalog(t, d.Repo)
	f.Expect([]string{"npx", "caveman-shrink", "--with-mcp-shrink=npx -y @mcp/seq"}, "", nil)
	step := &caveShrinkStep{d: d}
	if err := runStep(t, step); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	for _, c := range f.Calls() {
		if len(c.Argv) > 2 && c.Argv[0] == "npx" && c.Argv[1] == "caveman-shrink" {
			if strings.Contains(c.Argv[2], "https://mcp.ctx.com") {
				t.Error("Run invoked caveman-shrink for http transport entry (only stdio must be shrunk)")
			}
		}
	}
}
