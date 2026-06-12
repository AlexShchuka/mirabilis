package hooks

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/provision"
)

func startFakeProxy8787(t *testing.T) (stop func(), ok bool) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:8787")
	if err != nil {
		return func() {}, false
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck
	return func() { srv.Close() }, true
}

func writeHooksSettings(t *testing.T, path string, m map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readHooksSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var m map[string]any
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSessionStartBaseURL_SetsKey(t *testing.T) {
	tmp := t.TempDir()
	sp := filepath.Join(tmp, "settings.json")
	writeHooksSettings(t, sp, map[string]any{"theme": "dark"})

	sessionStartBaseURL(sp)

	m := readHooksSettings(t, sp)
	env, _ := m["env"].(map[string]any)
	if env == nil || env[provision.HeadroomBaseURLKey] != provision.HeadroomProxyURL {
		t.Errorf("ANTHROPIC_BASE_URL not set; env = %v", env)
	}
	if m["theme"] != "dark" {
		t.Errorf("theme key lost; m = %v", m)
	}
}

func TestSessionStartBaseURL_MissingFile_Noop(t *testing.T) {
	sessionStartBaseURL(filepath.Join(t.TempDir(), "nonexistent.json"))
}

func TestSessionRemoveBaseURL_RemovesKey(t *testing.T) {
	tmp := t.TempDir()
	sp := filepath.Join(tmp, "settings.json")
	writeHooksSettings(t, sp, map[string]any{
		"env": map[string]any{provision.HeadroomBaseURLKey: provision.HeadroomProxyURL, "OTHER": "val"},
	})

	sessionRemoveBaseURL(sp)

	m := readHooksSettings(t, sp)
	env, _ := m["env"].(map[string]any)
	if env != nil && env[provision.HeadroomBaseURLKey] != nil {
		t.Errorf("ANTHROPIC_BASE_URL should be removed; env = %v", env)
	}
	if env == nil || env["OTHER"] != "val" {
		t.Errorf("OTHER key should be preserved; env = %v", env)
	}
}

func TestSessionRemoveBaseURL_KeyAbsent_Noop(t *testing.T) {
	tmp := t.TempDir()
	sp := filepath.Join(tmp, "settings.json")
	writeHooksSettings(t, sp, map[string]any{"theme": "dark"})

	sessionRemoveBaseURL(sp)

	m := readHooksSettings(t, sp)
	if m["theme"] != "dark" {
		t.Errorf("settings mutated unexpectedly: %v", m)
	}
}

func TestSessionRemoveBaseURL_MissingFile_Noop(t *testing.T) {
	sessionRemoveBaseURL(filepath.Join(t.TempDir(), "nonexistent.json"))
}

func TestProxyAlive_WithFakeServer_ReturnsTrue(t *testing.T) {
	stop, ok := startFakeProxy8787(t)
	if !ok {
		t.Skip("port 8787 already in use; skipping")
	}
	defer stop()

	if !proxyAlive() {
		t.Error("proxyAlive = false with fake server on 8787, want true")
	}
}

func TestEnsureProxyForSession_BinaryAbsent_StaleKeyRemoved(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	sp := filepath.Join(tmp, ".claude", "settings.json")
	writeHooksSettings(t, sp, map[string]any{
		"env": map[string]any{provision.HeadroomBaseURLKey: provision.HeadroomProxyURL},
	})

	ensureProxyForSession()

	m := readHooksSettings(t, sp)
	env, _ := m["env"].(map[string]any)
	if env != nil && env[provision.HeadroomBaseURLKey] != nil {
		t.Errorf("stale ANTHROPIC_BASE_URL not removed when binary absent; env = %v", env)
	}
}

func TestEnsureProxyForSession_ProxyAlive_SetsBaseURL(t *testing.T) {
	stop, ok := startFakeProxy8787(t)
	if !ok {
		t.Skip("port 8787 already in use; skipping")
	}
	defer stop()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	sp := filepath.Join(tmp, ".claude", "settings.json")
	writeHooksSettings(t, sp, map[string]any{"theme": "dark"})

	ensureProxyForSession()

	m := readHooksSettings(t, sp)
	env, _ := m["env"].(map[string]any)
	if env == nil || env[provision.HeadroomBaseURLKey] != provision.HeadroomProxyURL {
		t.Errorf("ANTHROPIC_BASE_URL not set when proxy alive; env = %v", env)
	}
}

func TestSessionStartBaseURL_InvalidJSON_Noop(t *testing.T) {
	tmp := t.TempDir()
	sp := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(sp, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	sessionStartBaseURL(sp)
}

func TestSessionRemoveBaseURL_InvalidJSON_Noop(t *testing.T) {
	tmp := t.TempDir()
	sp := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(sp, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	sessionRemoveBaseURL(sp)
}
