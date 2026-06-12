package steps

import (
	"errors"
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/claudeauth"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

func newClaudeAuthForTest(t *testing.T, store *fakeStore) *claudeAuthStep {
	t.Helper()
	return &claudeAuthStep{d: newTestDeps(t, exec.NewFake(), sandbox.NewFakeDocker(), store)}
}

func TestClaudeAuthCheck(t *testing.T) {
	t.Parallel()
	t.Run("token present", func(t *testing.T) {
		t.Parallel()
		store := newFakeStore()
		store.m["claude-token"] = "sk-ant-oat01-abc"
		mustCheck(t, newClaudeAuthForTest(t, store), true)
	})
	t.Run("token missing", func(t *testing.T) {
		t.Parallel()
		mustCheck(t, newClaudeAuthForTest(t, newFakeStore()), false)
	})
}

func TestClaudeAuthRun(t *testing.T) {
	t.Parallel()
	s := newClaudeAuthForTest(t, newFakeStore())
	evs, err := runStep(t, s, func(any) pipeline.Result { return pipeline.Result{} })
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := waitingEvent(t, evs).Argv; !slices.Equal(got, claudeauth.SetupArgv()) {
		t.Fatalf("waiting argv = %v, want %v", got, claudeauth.SetupArgv())
	}
}

func TestClaudeAuthRunCancelled(t *testing.T) {
	t.Parallel()
	s := newClaudeAuthForTest(t, newFakeStore())
	_, err := runStep(t, s, func(any) pipeline.Result { return pipeline.Result{Cancelled: true} })
	if !errors.Is(err, pipeline.ErrCancelled) {
		t.Fatalf("err = %v, want ErrCancelled", err)
	}
}
