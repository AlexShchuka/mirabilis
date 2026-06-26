package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func TestParseStopInput(t *testing.T) {
	in := parseStopInput([]byte(`{"session_id":"abc","transcript_path":"/t.jsonl","hook_event_name":"Stop"}`))
	if in.SessionID != "abc" {
		t.Errorf("SessionID = %q, want abc", in.SessionID)
	}
	if in.TranscriptPath != "/t.jsonl" {
		t.Errorf("TranscriptPath = %q, want /t.jsonl", in.TranscriptPath)
	}
	if got := parseStopInput(nil); got.SessionID != "" {
		t.Errorf("parseStopInput(nil) = %+v, want zero", got)
	}
}

func TestTranscriptMinusToolsStripsTools(t *testing.T) {
	lines := []string{
		`{"type":"queue-operation","operation":"enqueue"}`,
		`{"type":"user","message":{"role":"user","content":"please review the diff"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"Here is my review."}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{}}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"file bytes"}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"No findings."}]}}`,
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "t.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := transcriptMinusTools(path)

	for _, want := range []string{
		"user: please review the diff",
		"assistant: Here is my review.",
		"assistant: No findings.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("transcript missing %q in:\n%s", want, got)
		}
	}
	for _, banned := range []string{"tool_use", "tool_result", "file bytes", "Read", "thinking", "hmm"} {
		if strings.Contains(got, banned) {
			t.Errorf("transcript should have stripped %q, got:\n%s", banned, got)
		}
	}
}

func TestTranscriptMinusToolsMissingPath(t *testing.T) {
	if got := transcriptMinusTools(""); got != "" {
		t.Errorf("transcriptMinusTools(\"\") = %q, want empty", got)
	}
	if got := transcriptMinusTools(filepath.Join(t.TempDir(), "nope.jsonl")); got != "" {
		t.Errorf("transcriptMinusTools(missing) = %q, want empty", got)
	}
}

func TestCollectReposTableOverFakeRunner(t *testing.T) {
	hdir := t.TempDir()
	t.Setenv("HOME", hdir)
	root := filepath.Join(hdir, ecosystemDirRel)

	changed := filepath.Join(root, "darwin")
	clean := filepath.Join(root, "mirabilis")
	notRepo := filepath.Join(root, "loose")
	for _, d := range []string{changed, clean, notRepo} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(changed, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(clean, ".git", "HEAD"), "ref: refs/heads/main\n")

	f := exec.NewFake()
	f.Expect([]string{"git", "-C", changed, "rev-parse", "--git-dir"}, ".git", nil)
	f.Expect([]string{"git", "-C", changed, "diff", "HEAD"}, "diff --git a/file.txt b/file.txt\n+new line\n", nil)
	f.Expect([]string{"git", "-C", changed, "status", "--porcelain"}, " M file.txt\n?? added.txt\n", nil)
	f.Expect([]string{"git", "-C", clean, "rev-parse", "--git-dir"}, ".git", nil)
	f.Expect([]string{"git", "-C", clean, "diff", "HEAD"}, "", nil)
	f.Expect([]string{"git", "-C", clean, "status", "--porcelain"}, "", nil)
	setRunner(t, f)

	repos := collectRepos(t.Context())

	if len(repos) != 1 {
		t.Fatalf("collectRepos returned %d repos, want 1 (changed only)", len(repos))
	}
	r := repos[0]
	if r.Name != "darwin" {
		t.Errorf("repo name = %q, want darwin", r.Name)
	}
	if r.Dir != changed {
		t.Errorf("repo dir = %q, want %q", r.Dir, changed)
	}
	if !strings.Contains(r.Diff, "+new line") {
		t.Errorf("repo diff = %q, want it to contain the diff", r.Diff)
	}
	wantFiles := []string{"file.txt", "added.txt"}
	if len(r.Files) != len(wantFiles) {
		t.Fatalf("repo files = %v, want %v", r.Files, wantFiles)
	}
	for i, w := range wantFiles {
		if r.Files[i] != w {
			t.Errorf("repo files[%d] = %q, want %q", i, r.Files[i], w)
		}
	}
	if rem := f.Remaining(); rem != 0 {
		t.Errorf("collectRepos left %d unused stubs", rem)
	}
}

func TestCollectReposNoRoot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if repos := collectRepos(t.Context()); repos != nil {
		t.Errorf("collectRepos with no ecosystem dir = %v, want nil", repos)
	}
}

func TestHarvestWritesManifest(t *testing.T) {
	hdir := t.TempDir()
	t.Setenv("HOME", hdir)
	root := filepath.Join(hdir, ecosystemDirRel)

	changed := filepath.Join(root, "darwin")
	if err := os.MkdirAll(changed, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(changed, ".git", "HEAD"), "ref: refs/heads/main\n")

	transcript := filepath.Join(hdir, "t.jsonl")
	if err := os.WriteFile(transcript, []byte(
		`{"type":"user","message":{"role":"user","content":"do the thing"}}`+"\n"+
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash"}]}}`+"\n"+
			`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"done"}]}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := exec.NewFake()
	f.Expect([]string{"git", "-C", changed, "rev-parse", "--git-dir"}, ".git", nil)
	f.Expect([]string{"git", "-C", changed, "diff", "HEAD"}, "diff body", nil)
	f.Expect([]string{"git", "-C", changed, "status", "--porcelain"}, " M a.txt\n", nil)
	setRunner(t, f)

	in := stopHookInput{SessionID: "sess-1", TranscriptPath: transcript}
	m := harvest(t.Context(), in, "2026-06-26T00:00:00Z")
	path, err := writeManifest(m)
	if err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	if path != filepath.Join(hdir, ".claude", changesetManifestFileName) {
		t.Errorf("manifest path = %q, want stable ~/.claude path", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var round Manifest
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if round.SessionID != "sess-1" {
		t.Errorf("sessionId = %q, want sess-1", round.SessionID)
	}
	if round.Timestamp != "2026-06-26T00:00:00Z" {
		t.Errorf("timestamp = %q, want fixed stamp", round.Timestamp)
	}
	if len(round.Repos) != 1 || round.Repos[0].Name != "darwin" {
		t.Fatalf("repos = %+v, want one darwin repo", round.Repos)
	}
	if round.Repos[0].Diff != "diff body" {
		t.Errorf("repo diff = %q, want \"diff body\"", round.Repos[0].Diff)
	}
	tmt := round.Session.TranscriptMinusTools
	if !strings.Contains(tmt, "user: do the thing") || !strings.Contains(tmt, "assistant: done") {
		t.Errorf("transcriptMinusTools missing kept text: %q", tmt)
	}
	if strings.Contains(tmt, "tool_use") || strings.Contains(tmt, "Bash") {
		t.Errorf("transcriptMinusTools should have stripped tools: %q", tmt)
	}
}
