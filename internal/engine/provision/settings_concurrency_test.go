package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func TestUpdateJSONConcurrentNoLostUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	mustWriteJSON(t, path, map[string]any{})

	const writers = 16
	var start sync.WaitGroup
	start.Add(1)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			start.Wait()
			key := fmt.Sprintf("key%d", n)
			if err := updateJSON(path, func(m map[string]any) error {
				time.Sleep(time.Millisecond)
				m[key] = n
				return nil
			}); err != nil {
				t.Errorf("updateJSON: %v", err)
			}
		}(i)
	}
	start.Done()
	wg.Wait()

	m := mustReadJSON(t, path)
	for i := range writers {
		key := fmt.Sprintf("key%d", i)
		if _, ok := m[key]; !ok {
			t.Errorf("lost update: %s missing from %v", key, m)
		}
	}
}

func TestSettingsWritersConcurrentNoLostUpdate(t *testing.T) {
	d, _ := testDeps(t)
	d.SessionKey = "sk-concurrent"

	mustWrite(t, filepath.Join(d.Repo, "config", "loadouts", "raid.txt"), "effort max\nharness on\n")
	if err := config.WriteLoadout(d.Repo, "raid"); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, d.themePath(), "dark\n")
	mustWriteJSON(t, d.settingsPath(), map[string]any{"seed": "present"})

	writers := []pipeline.Command{
		&effortStep{d: d},
		&themeStep{d: d},
		&onboardingStep{d: d},
		&settingsEnvStep{d: d},
	}

	const rounds = 20
	var start sync.WaitGroup
	start.Add(1)
	var wg sync.WaitGroup
	for range rounds {
		for _, w := range writers {
			wg.Add(1)
			go func(step pipeline.Command) {
				defer wg.Done()
				start.Wait()
				out := make(chan pipeline.Event, 8)
				done := make(chan struct{})
				go func() {
					defer close(done)
					for range out {
					}
				}()
				if err := step.Run(t.Context(), out, nil); err != nil {
					t.Errorf("%s run: %v", step.Meta().Name, err)
				}
				close(out)
				<-done
			}(w)
		}
	}
	start.Done()
	wg.Wait()

	m := mustReadJSON(t, d.settingsPath())
	if m["effortLevel"] != "max" {
		t.Errorf("effortLevel lost: got %v, want max", m["effortLevel"])
	}
	if m["theme"] != "dark" {
		t.Errorf("theme lost: got %v, want dark", m["theme"])
	}
	if sdpp, _ := m["skipDangerousModePermissionPrompt"].(bool); !sdpp {
		t.Errorf("skipDangerousModePermissionPrompt lost: got %v, want true", m["skipDangerousModePermissionPrompt"])
	}
	env, _ := m[settingsEnvKey].(map[string]any)
	if env == nil || env[settingsTokenKey] != d.SessionKey {
		t.Errorf("auth-chain env lost: got %v", m[settingsEnvKey])
	}
	if m["seed"] != "present" {
		t.Errorf("pre-existing seed key lost: got %v", m["seed"])
	}
}

func TestSettingsFreshCreateConcurrentNoLostUpdate(t *testing.T) {
	d, _ := testDeps(t)
	mustWriteJSON(t, d.Cfg.SettingsSeed(), map[string]any{"seedMarker": "present", "theme": "auto"})

	const rounds = 50
	for r := range rounds {
		if err := os.Remove(d.settingsPath()); err != nil && !os.IsNotExist(err) {
			t.Fatalf("round %d: remove settings: %v", r, err)
		}
		var start sync.WaitGroup
		start.Add(1)
		var wg sync.WaitGroup
		for _, step := range []pipeline.Command{&settingsStep{d: d}, &onboardingStep{d: d}} {
			wg.Add(1)
			go func(s pipeline.Command) {
				defer wg.Done()
				start.Wait()
				if err := runStep(t, s); err != nil {
					t.Errorf("round %d %s: %v", r, s.Meta().Name, err)
				}
			}(step)
		}
		start.Done()
		wg.Wait()

		m := mustReadJSON(t, d.settingsPath())
		if m["seedMarker"] != "present" {
			t.Fatalf("round %d: settingsStep seed lost racing onboarding: %v", r, m)
		}
		if sdpp, _ := m["skipDangerousModePermissionPrompt"].(bool); !sdpp {
			t.Fatalf("round %d: onboarding skipDangerous lost racing settings fresh-create: %v", r, m)
		}
	}
}
