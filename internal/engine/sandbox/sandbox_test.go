package sandbox

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"sync"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

type recordingRunner struct {
	fake *exec.Fake

	mu    sync.Mutex
	specs []exec.Spec
}

func (r *recordingRunner) Stream(ctx context.Context, spec exec.Spec) <-chan exec.Event {
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()
	return r.fake.Stream(ctx, spec)
}

func (r *recordingRunner) composeSpec(t *testing.T) exec.Spec {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, spec := range r.specs {
		if len(spec.Argv) > 1 && spec.Argv[0] == "docker" && spec.Argv[1] == "compose" {
			return spec
		}
	}
	t.Fatal("no compose spawn recorded")
	return exec.Spec{}
}

func gitArgv(repo string) []string {
	return []string{"git", "-C", repo, "rev-parse", "--short", "HEAD"}
}

func TestComposeArgv(t *testing.T) {
	t.Parallel()
	ops := []struct {
		name string
		call func(*Sandbox, context.Context) <-chan exec.Event
		tail []string
	}{
		{"build", (*Sandbox).Build, []string{"build"}},
		{"up", (*Sandbox).Up, []string{"up", "-d"}},
		{"down", (*Sandbox).Down, []string{"down"}},
		{"reset", (*Sandbox).Reset, []string{"down", "--rmi", "local", "-v"}},
	}
	for _, sock := range []bool{false, true} {
		for _, op := range ops {
			t.Run(op.name, func(t *testing.T) {
				t.Parallel()
				repo := t.TempDir()
				if err := config.WriteSock(repo, sock); err != nil {
					t.Fatal(err)
				}
				fake := exec.NewFake().
					Expect([]string{"git"}, "abc1234", nil).
					Expect([]string{"docker", "compose"}, "", nil)
				s := New(fake, NewFakeDocker(), repo)
				if err := drain(op.call(s, context.Background())); err != nil {
					t.Fatal(err)
				}
				calls := fake.Calls()
				if len(calls) != 2 {
					t.Fatalf("got %d calls, want 2: %v", len(calls), calls)
				}
				if !slices.Equal(calls[0].Argv, gitArgv(repo)) {
					t.Fatalf("git argv = %v", calls[0].Argv)
				}
				want := []string{"docker", "compose", "-f", "docker-compose.yml"}
				if sock {
					want = append(want, "-f", "compose.sock.yml")
				}
				want = append(want, op.tail...)
				if !slices.Equal(calls[1].Argv, want) {
					t.Fatalf("compose argv = %v, want %v", calls[1].Argv, want)
				}
				if calls[1].Dir != repo {
					t.Fatalf("compose dir = %q, want %q", calls[1].Dir, repo)
				}
			})
		}
	}
}

func TestComposeEnv(t *testing.T) {
	repo := t.TempDir()
	if err := config.WriteStacks(repo, "go,python"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TZ", "Europe/Berlin")
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "sk-ant-oat01-secret")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tg-secret")
	t.Setenv("TELEGRAM_CHAT_ID", "42")
	rec := &recordingRunner{fake: exec.NewFake().
		Expect([]string{"git"}, "abc1234", nil).
		Expect([]string{"docker", "compose"}, "", nil)}
	s := New(rec, NewFakeDocker(), repo)
	if err := drain(s.Up(context.Background())); err != nil {
		t.Fatal(err)
	}
	env := rec.composeSpec(t).Env
	for _, want := range []string{
		"MIRABILIS_VERSION=abc1234-go,python",
		"STACKS=go,python",
		"TZ=Europe/Berlin",
	} {
		if !slices.Contains(env, want) {
			t.Errorf("env missing %q: %v", want, env)
		}
	}
	forbidden := regexp.MustCompile(`(?i)^[^=]*(claude|telegram|token|secret)[^=]*=`)
	for _, kv := range env {
		if forbidden.MatchString(kv) {
			t.Errorf("forbidden env entry %q", kv)
		}
	}
}

func TestDesiredFingerprint(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := config.WriteStacks(repo, "go,python"); err != nil {
		t.Fatal(err)
	}
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc1234", nil).
		Expect([]string{"git"}, "abc1234", nil)
	s := New(fake, NewFakeDocker(), repo)
	first := s.Desired(context.Background())
	second := s.Desired(context.Background())
	if first != "abc1234-go,python" {
		t.Fatalf("fingerprint = %q", first)
	}
	if first != second {
		t.Fatalf("fingerprint not deterministic: %q vs %q", first, second)
	}
}

func TestDesiredSockFlipsFingerprint(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := config.WriteStacks(repo, "go,python"); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteSock(repo, true); err != nil {
		t.Fatal(err)
	}
	fake := exec.NewFake().Expect([]string{"git"}, "abc1234", nil)
	s := New(fake, NewFakeDocker(), repo)
	if got := s.Desired(context.Background()); got != "abc1234-go,python-sock" {
		t.Fatalf("fingerprint = %q, want %q", got, "abc1234-go,python-sock")
	}
}

func TestDesiredStacksFlipFingerprint(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc1234", nil).
		Expect([]string{"git"}, "abc1234", nil)
	s := New(fake, NewFakeDocker(), repo)
	if err := config.WriteStacks(repo, "go"); err != nil {
		t.Fatal(err)
	}
	one := s.Desired(context.Background())
	if err := config.WriteStacks(repo, "go,python"); err != nil {
		t.Fatal(err)
	}
	two := s.Desired(context.Background())
	if one == two {
		t.Fatalf("STACKS change did not flip fingerprint: %q", one)
	}
}

func TestDesiredGitUnavailable(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	fake := exec.NewFake().Expect([]string{"git"}, "", errors.New("not a repo"))
	s := New(fake, NewFakeDocker(), repo)
	if got := s.Desired(context.Background()); got != "unknown-" {
		t.Fatalf("fingerprint = %q, want %q", got, "unknown-")
	}
}

func TestRunningAndStale(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	if err := config.WriteStacks(repo, "go,python"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	t.Run("match", func(t *testing.T) {
		t.Parallel()
		fd := NewFakeDocker().StubInspect(Container{
			Running: true,
			Env:     map[string]string{"MIRABILIS_VERSION": "abc1234-go,python"},
		}, nil)
		fake := exec.NewFake().Expect([]string{"git"}, "abc1234", nil)
		s := New(fake, fd, repo)
		if got := s.Running(ctx); got != "abc1234-go,python" {
			t.Fatalf("Running = %q", got)
		}
		if s.Stale(ctx) {
			t.Fatal("Stale = true, want false")
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		t.Parallel()
		fd := NewFakeDocker().StubInspect(Container{
			Running: true,
			Env:     map[string]string{"MIRABILIS_VERSION": "old0000-go,python"},
		}, nil)
		fake := exec.NewFake().Expect([]string{"git"}, "abc1234", nil)
		s := New(fake, fd, repo)
		if !s.Stale(ctx) {
			t.Fatal("Stale = false, want true")
		}
	})

	t.Run("not running", func(t *testing.T) {
		t.Parallel()
		fd := NewFakeDocker().StubInspect(Container{Running: false}, nil)
		s := New(exec.NewFake(), fd, repo)
		if got := s.Running(ctx); got != "" {
			t.Fatalf("Running = %q, want empty", got)
		}
		if !s.Stale(ctx) {
			t.Fatal("Stale = false, want true")
		}
	})

	t.Run("daemon down", func(t *testing.T) {
		t.Parallel()
		fd := NewFakeDocker().StubInspect(Container{}, errors.New("daemon down"))
		s := New(exec.NewFake(), fd, repo)
		if got := s.Running(ctx); got != "" {
			t.Fatalf("Running = %q, want empty", got)
		}
		if !s.Stale(ctx) {
			t.Fatal("Stale = false, want true")
		}
	})
}
