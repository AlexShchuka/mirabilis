package steps

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func newFormsDeps(t *testing.T) Deps {
	t.Helper()
	d := newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), newFakeStore())
	dir := filepath.Join(d.Repo, "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"stacks.txt":  "rust\nelixir\n",
		"plugins.txt": "alpha\nbeta\ngamma\n",
		"skills.txt":  "writer\nresearcher\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func resolveStrings(v []string) func(any) pipeline.Result {
	return func(any) pipeline.Result { return pipeline.Result{Value: v} }
}

func TestStacksForm(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	s := newStacksForm(d)
	mustCheck(t, s, false)
	evs, err := runStep(t, s, resolveStrings([]string{"rust"}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	cat, ok := waitingEvent(t, evs).Payload.(Catalog)
	if !ok {
		t.Fatalf("payload = %T, want Catalog", waitingEvent(t, evs).Payload)
	}
	if !slices.Equal(cat.Options, []string{"rust", "elixir"}) || !cat.MultiSelect {
		t.Fatalf("catalog = %+v", cat)
	}
	if v, ok := config.ReadStacks(d.Repo); !ok || v != "rust" {
		t.Fatalf("persisted stacks = %q ok=%v, want rust", v, ok)
	}
	mustCheck(t, s, true)
}

func TestStacksFormPersistedEmptyCountsAsPersisted(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	if err := config.WriteStacks(d.Repo, ""); err != nil {
		t.Fatal(err)
	}
	mustCheck(t, newStacksForm(d), true)
}

func TestPluginsFormSelectionIsComplementOfDisabled(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	if err := config.WritePluginsDisabled(d.Repo, []string{"beta"}); err != nil {
		t.Fatal(err)
	}
	s := newPluginsForm(d)
	mustCheck(t, s, true)
	evs, err := runStep(t, s, resolveStrings([]string{"alpha"}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	cat := waitingEvent(t, evs).Payload.(Catalog)
	if !slices.Equal(cat.Selected, []string{"alpha", "gamma"}) {
		t.Fatalf("selected = %v, want enabled complement", cat.Selected)
	}
	if got := config.ReadPluginsDisabled(d.Repo); !slices.Equal(got, []string{"beta", "gamma"}) {
		t.Fatalf("disabled = %v, want [beta gamma]", got)
	}
}

func TestPluginsFormCheckRequiresKey(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	s := newPluginsForm(d)
	mustCheck(t, s, false)
	if err := config.WritePluginsDisabled(d.Repo, nil); err != nil {
		t.Fatal(err)
	}
	mustCheck(t, s, true)
}

func TestSkillsForm(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	s := newSkillsForm(d)
	mustCheck(t, s, false)
	if _, err := runStep(t, s, resolveStrings([]string{"writer", "researcher"})); err != nil {
		t.Fatalf("run: %v", err)
	}
	if v, _ := config.ReadSkills(d.Repo); v != "writer,researcher" {
		t.Fatalf("persisted skills = %q", v)
	}
	mustCheck(t, s, true)
}

func TestFormCancelled(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	_, err := runStep(t, newStacksForm(d), func(any) pipeline.Result { return pipeline.Result{Cancelled: true} })
	if !errors.Is(err, pipeline.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	if _, ok := config.ReadStacks(d.Repo); ok {
		t.Fatal("cancelled form must not persist")
	}
}

func TestFormRejectsWrongResultType(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	_, err := runStep(t, newStacksForm(d), func(any) pipeline.Result { return pipeline.Result{Value: 42} })
	if err == nil || !strings.Contains(err.Error(), "expected []string") {
		t.Fatalf("err = %v, want type error", err)
	}
}
