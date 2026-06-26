package steps

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

type staticRunner struct {
	stubs map[string]string
}

var _ exec.Runner = (*staticRunner)(nil)

func (r *staticRunner) Stream(_ context.Context, spec exec.Spec) <-chan exec.Event {
	ch := make(chan exec.Event, 8)
	go func() {
		defer close(ch)
		ch <- exec.Event{Kind: exec.KindStarted, Argv: spec.Argv}
		key := strings.Join(spec.Argv, " ")
		out, ok := r.stubs[key]
		if !ok {
			ch <- exec.Event{Kind: exec.KindExited, Code: 1, Err: errUnstubbed}
			return
		}
		for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if line != "" {
				ch <- exec.Event{Kind: exec.KindStdout, Line: line}
			}
		}
		ch <- exec.Event{Kind: exec.KindExited}
	}()
	return ch
}

var errUnstubbed = newErr("staticRunner: unstubbed command")

func newErr(msg string) error { return &simpleErr{msg} }

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }

func reviewHome(t *testing.T) (home, root string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	root = filepath.Join(home, ecosystemDirRel)
	return home, root
}

func mkGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestReviewStepSetHasNoLaunchSteps(t *testing.T) {
	d := newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore())
	cmds := Review(d)
	if len(cmds) != 2 {
		t.Fatalf("Review returned %d steps, want 2", len(cmds))
	}
	got := []string{cmds[0].Meta().Name, cmds[1].Meta().Name}
	want := []string{reviewHarvestName, reviewPresentName}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	for _, c := range cmds {
		for _, banned := range []string{"container", "provision-create", "provision-start", "attach", "image"} {
			if c.Meta().Name == banned {
				t.Errorf("Review must not include launch step %q", banned)
			}
		}
	}
}

func TestReviewHarvestIdempotencyContract(t *testing.T) {
	_, root := reviewHome(t)
	changed := filepath.Join(root, "darwin")
	clean := filepath.Join(root, "mirabilis")
	mkGitRepo(t, changed)
	mkGitRepo(t, clean)

	sr := &staticRunner{stubs: map[string]string{
		"git -C " + changed + " rev-parse --git-dir": ".git",
		"git -C " + changed + " diff HEAD":           "diff --git a/x b/x\n+new",
		"git -C " + changed + " status --porcelain":  " M x\n",
		"git -C " + clean + " rev-parse --git-dir":   ".git",
		"git -C " + clean + " diff HEAD":             "",
		"git -C " + clean + " status --porcelain":    "",
	}}
	d := newTestDeps(t, sr, sandbox.NewFakeDocker(), newFakeStore())

	step := &reviewHarvestStep{d: d}
	pipeline.Contract(t, step, nil)
}

func TestReviewHarvestWritesManifestAndPresent(t *testing.T) {
	home, root := reviewHome(t)
	changed := filepath.Join(root, "darwin")
	mkGitRepo(t, changed)

	sr := &staticRunner{stubs: map[string]string{
		"git -C " + changed + " rev-parse --git-dir": ".git",
		"git -C " + changed + " diff HEAD":           "diff body",
		"git -C " + changed + " status --porcelain":  " M a.txt\n?? b.txt\n",
	}}
	d := newTestDeps(t, sr, sandbox.NewFakeDocker(), newFakeStore())

	harvest := &reviewHarvestStep{d: d}
	if _, err := runStep(t, harvest, nil); err != nil {
		t.Fatalf("harvest run: %v", err)
	}

	manifestFile := filepath.Join(home, ".claude", changesetManifestFileName)
	data, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m changesetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("manifest not valid JSON: %v", err)
	}
	if len(m.Repos) != 1 || m.Repos[0].Name != "darwin" {
		t.Fatalf("repos = %+v, want one darwin repo", m.Repos)
	}
	if m.Repos[0].Diff != "diff body" {
		t.Errorf("diff = %q, want \"diff body\"", m.Repos[0].Diff)
	}
	wantFiles := []string{"a.txt", "b.txt"}
	if len(m.Repos[0].Files) != len(wantFiles) {
		t.Fatalf("files = %v, want %v", m.Repos[0].Files, wantFiles)
	}

	present := &reviewPresentStep{d: d}
	evs, err := runStep(t, present, nil)
	if err != nil {
		t.Fatalf("present run: %v", err)
	}
	var lines []string
	for _, ev := range evs {
		if ev.Kind == pipeline.EvLine {
			lines = append(lines, ev.Line)
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "darwin: 2 changed file(s)") {
		t.Errorf("present output missing per-repo summary, got: %q", joined)
	}
}

func TestReviewHarvestNoEcosystem(t *testing.T) {
	reviewHome(t)
	d := newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore())
	harvest := &reviewHarvestStep{d: d}
	if _, err := runStep(t, harvest, nil); err != nil {
		t.Fatalf("harvest with no ecosystem dir: %v", err)
	}
	home := os.Getenv("HOME")
	data, err := os.ReadFile(filepath.Join(home, ".claude", changesetManifestFileName))
	if err != nil {
		t.Fatalf("manifest should still be written: %v", err)
	}
	var m changesetManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("manifest not valid JSON: %v", err)
	}
	if len(m.Repos) != 0 {
		t.Errorf("repos = %+v, want empty", m.Repos)
	}
}
