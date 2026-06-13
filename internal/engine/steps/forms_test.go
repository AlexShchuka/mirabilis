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

func resolveWizard(choices map[string][]string) func(any) pipeline.Result {
	return func(any) pipeline.Result {
		return pipeline.Result{Value: WizardResult{Choices: choices}}
	}
}

func wizardEvent(t *testing.T, evs []pipeline.Event) Wizard {
	t.Helper()
	w, ok := waitingEvent(t, evs).Payload.(Wizard)
	if !ok {
		t.Fatalf("payload = %T, want Wizard", waitingEvent(t, evs).Payload)
	}
	return w
}

func groupByKey(t *testing.T, w Wizard, key string) Catalog {
	t.Helper()
	for _, c := range w.Groups {
		if c.Key == key {
			return c
		}
	}
	t.Fatalf("group %q not present in wizard %+v", key, w.Groups)
	return Catalog{}
}

func TestConfigWizardEmitsAllThreeGroupsKeyed(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	s := newConfig(d)
	mustCheck(t, s, false)
	evs, err := runStep(t, s, resolveWizard(map[string][]string{
		keyStacks:  {"rust"},
		keyPlugins: {"alpha"},
		keySkills:  {"writer", "researcher"},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	w := wizardEvent(t, evs)
	if len(w.Groups) != 3 {
		t.Fatalf("groups = %d, want 3", len(w.Groups))
	}
	if got := []string{w.Groups[0].Key, w.Groups[1].Key, w.Groups[2].Key}; !slices.Equal(got, []string{keyStacks, keyPlugins, keySkills}) {
		t.Fatalf("group order = %v, want [stacks plugins skills]", got)
	}
	for _, c := range w.Groups {
		if c.Description == "" {
			t.Errorf("group %q: empty Description (Scan-not-Read)", c.Key)
		}
		if !c.MultiSelect {
			t.Errorf("group %q: MultiSelect false", c.Key)
		}
	}
	if v, ok := config.ReadStacks(d.Repo); !ok || v != "rust" {
		t.Fatalf("stacks = %q ok=%v, want rust", v, ok)
	}
	if got := config.ReadPluginsDisabled(d.Repo); !slices.Equal(got, []string{"beta", "gamma"}) {
		t.Fatalf("plugins disabled = %v, want [beta gamma]", got)
	}
	if v, _ := config.ReadSkills(d.Repo); v != "writer,researcher" {
		t.Fatalf("skills = %q, want writer,researcher", v)
	}
	mustCheck(t, s, true)
}

func TestConfigWizardPluginsSelectionIsComplementOfDisabled(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	if err := config.WritePluginsDisabled(d.Repo, []string{"beta"}); err != nil {
		t.Fatal(err)
	}
	evs, err := runStep(t, newConfig(d), resolveWizard(map[string][]string{
		keyStacks:  nil,
		keyPlugins: {"alpha"},
		keySkills:  nil,
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	plugins := groupByKey(t, wizardEvent(t, evs), keyPlugins)
	if !slices.Equal(plugins.Selected, []string{"alpha", "gamma"}) {
		t.Fatalf("preselected = %v, want enabled complement [alpha gamma]", plugins.Selected)
	}
	if got := config.ReadPluginsDisabled(d.Repo); !slices.Equal(got, []string{"beta", "gamma"}) {
		t.Fatalf("disabled = %v, want [beta gamma]", got)
	}
}

func TestConfigCheckRequiresAllThreePersisted(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	s := newConfig(d)
	mustCheck(t, s, false)
	if err := config.WriteStacks(d.Repo, "rust"); err != nil {
		t.Fatal(err)
	}
	mustCheck(t, s, false)
	if err := config.WritePluginsDisabled(d.Repo, nil); err != nil {
		t.Fatal(err)
	}
	mustCheck(t, s, false)
	if err := config.WriteSkills(d.Repo, "writer"); err != nil {
		t.Fatal(err)
	}
	mustCheck(t, s, true)
}

func TestConfigRepeatLaunchAllPersistedSkipsWithoutWaiting(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	if err := config.WriteStacks(d.Repo, "rust"); err != nil {
		t.Fatal(err)
	}
	if err := config.WritePluginsDisabled(d.Repo, nil); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteSkills(d.Repo, "writer"); err != nil {
		t.Fatal(err)
	}
	mustCheck(t, newConfig(d), true)
}

func TestConfigCancelledPersistsNothing(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	_, err := runStep(t, newConfig(d), func(any) pipeline.Result { return pipeline.Result{Cancelled: true} })
	if !errors.Is(err, pipeline.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
	if _, ok := config.ReadStacks(d.Repo); ok {
		t.Fatal("cancelled wizard must not persist")
	}
}

func TestConfigRejectsWrongResultType(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	_, err := runStep(t, newConfig(d), func(any) pipeline.Result { return pipeline.Result{Value: 42} })
	if err == nil || !strings.Contains(err.Error(), "WizardResult") {
		t.Fatalf("err = %v, want WizardResult type error", err)
	}
}

func TestConfigRejectsResultMissingGroup(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	_, err := runStep(t, newConfig(d), resolveWizard(map[string][]string{
		keyStacks:  {"rust"},
		keyPlugins: {"alpha"},
	}))
	if err == nil || !strings.Contains(err.Error(), "missing group") {
		t.Fatalf("err = %v, want missing-group error for absent skills key", err)
	}
}

func TestConfigEmptyGroupOmittedFromWizard(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	dir := filepath.Join(d.Repo, "config")
	if err := os.WriteFile(filepath.Join(dir, "skills.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	evs, err := runStep(t, newConfig(d), resolveWizard(map[string][]string{
		keyStacks:  {"rust"},
		keyPlugins: {"alpha"},
	}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	w := wizardEvent(t, evs)
	for _, c := range w.Groups {
		if c.Key == keySkills {
			t.Fatalf("empty skills group must be omitted, got %+v", w.Groups)
		}
	}
	if len(w.Groups) != 2 {
		t.Fatalf("groups = %d, want 2 (skills omitted)", len(w.Groups))
	}
	if v, ok := config.ReadSkills(d.Repo); !ok || v != "" {
		t.Fatalf("empty skills group must persist the empty choice: got %q ok=%v", v, ok)
	}
}

func TestConfigAllEmptySkipsWithoutWaitingAndPersistsEmpty(t *testing.T) {
	t.Parallel()
	d := newFormsDeps(t)
	dir := filepath.Join(d.Repo, "config")
	for _, name := range []string{"stacks.txt", "plugins.txt", "skills.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := newConfig(d)
	evs, err := runStep(t, s, func(any) pipeline.Result {
		t.Error("config emitted EvWaiting for all-empty catalogs; expected auto-skip")
		return pipeline.Result{}
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, ev := range evs {
		if ev.Kind == pipeline.EvWaiting {
			t.Fatalf("EvWaiting emitted for all-empty catalogs; expected auto-skip")
		}
	}
	if v, ok := config.ReadStacks(d.Repo); !ok || v != "" {
		t.Fatalf("stacks empty-skip must persist empty choice: %q ok=%v", v, ok)
	}
	if _, ok := dotenvRead(d.Repo, pluginsDisabledKey); !ok {
		t.Fatal("plugins empty-skip must persist the disabled key (I9)")
	}
	if v, ok := config.ReadSkills(d.Repo); !ok || v != "" {
		t.Fatalf("skills empty-skip must persist empty choice: %q ok=%v", v, ok)
	}
	mustCheck(t, s, true)
}
