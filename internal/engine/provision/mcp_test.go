package provision

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMalformedMCPCatalog(t *testing.T, repo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config", "mcp.json"), []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMCPStepCheckErrorOnMalformedCatalog(t *testing.T) {
	d, f := testDeps(t)
	writeMalformedMCPCatalog(t, d.Repo)
	f.Expect(script("command -v claude"), "", nil)
	step := &mcpStep{d: d}
	ok, err := step.Check(t.Context())
	if err == nil {
		t.Fatal("Check should return error for malformed mcp.json, got nil")
	}
	if ok {
		t.Error("Check should not return success when catalog is malformed")
	}
}

func TestMCPStepRunErrorOnMalformedCatalog(t *testing.T) {
	d, f := testDeps(t)
	writeMalformedMCPCatalog(t, d.Repo)
	f.Expect(script("command -v claude"), "", nil)
	step := &mcpStep{d: d}
	if err := runStep(t, step); err == nil {
		t.Fatal("Run should return error for malformed mcp.json, got nil")
	}
}
