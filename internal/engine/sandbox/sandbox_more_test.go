package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
)

func TestResolveCodeFromPATH(t *testing.T) {
	dir := t.TempDir()
	codePath := filepath.Join(dir, "code")
	if err := os.WriteFile(codePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := resolveCode()
	if err != nil {
		t.Fatalf("resolveCode with code in PATH: %v", err)
	}
	if got != codePath {
		t.Errorf("resolveCode = %q, want %q", got, codePath)
	}
}

func TestLookPathEmptyPathEntrySkipped(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "myprog")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", ":"+dir)
	got, err := lookPath("myprog")
	if err != nil {
		t.Fatalf("lookPath with leading empty segment: %v", err)
	}
	if got != bin {
		t.Errorf("lookPath = %q, want %q", got, bin)
	}
}

func TestLookPathFindsExecutable(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "mybinary")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got, err := lookPath("mybinary")
	if err != nil {
		t.Fatalf("lookPath: %v", err)
	}
	if got != bin {
		t.Errorf("lookPath = %q, want %q", got, bin)
	}
}

func TestLookPathNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := lookPath("no-such-binary-xyz")
	if err == nil {
		t.Fatal("lookPath = nil, want error")
	}
}

func TestLookPathSkipsDirectory(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "code")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	_, err := lookPath("code")
	if err == nil {
		t.Fatal("lookPath returned directory as executable, want error")
	}
}

func TestOpenVSCodeUpFails(t *testing.T) {
	code := fakeCodeBinary(t)
	repo := t.TempDir()
	fd := NewFakeDocker().StubInspect(Container{Running: false}, nil)
	upErr := errors.New("compose up failed")
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc1234", nil).
		Expect([]string{"docker", "compose"}, "", upErr)
	s := New(fake, fd, repo)
	err := s.OpenVSCode(context.Background())
	if err == nil {
		t.Fatal("OpenVSCode with failing Up = nil, want error")
	}
	_ = code
}

func TestOpenVSCodeInspectError(t *testing.T) {
	code := fakeCodeBinary(t)
	repo := t.TempDir()
	fd := NewFakeDocker().StubInspect(Container{}, errors.New("daemon down"))
	fake := exec.NewFake().
		Expect([]string{"git"}, "abc1234", nil).
		Expect([]string{"docker", "compose"}, "", nil).
		Expect([]string{code}, "", nil)
	s := New(fake, fd, repo)
	if err := s.OpenVSCode(context.Background()); err != nil {
		t.Fatalf("OpenVSCode with inspect error = %v, want nil (Up should run)", err)
	}
	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3: %v", len(calls), calls)
	}
	wantUp := []string{"docker", "compose", "-f", "docker-compose.yml", "up", "-d"}
	if !slices.Equal(calls[1].Argv, wantUp) {
		t.Fatalf("up argv = %v, want %v", calls[1].Argv, wantUp)
	}
}

func TestOpenVSCodeNoCodeBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	repo := t.TempDir()
	fd := NewFakeDocker().StubInspect(Container{Running: true}, nil)
	s := New(exec.NewFake(), fd, repo)
	err := s.OpenVSCode(context.Background())
	if err == nil {
		t.Fatal("OpenVSCode with no code binary = nil, want error")
	}
}

func TestFakeEventsEmitAndClose(t *testing.T) {
	t.Parallel()
	fe := NewFakeEvents()
	fe.Emit("start")
	fe.Emit("die")
	fe.Close()

	var got []string
	for ev := range fe.events {
		got = append(got, ev.Action)
	}
	if !slices.Equal(got, []string{"start", "die"}) {
		t.Errorf("events = %v, want [start die]", got)
	}
}

func TestFakeEventsFail(t *testing.T) {
	t.Parallel()
	fe := NewFakeEvents()
	sentinel := errors.New("fake error")
	fe.Fail(sentinel)

	select {
	case err := <-fe.errs:
		if err != sentinel {
			t.Errorf("error = %v, want %v", err, sentinel)
		}
	default:
		t.Fatal("no error received from Fail")
	}
}

func TestFakeDockerStubEvents(t *testing.T) {
	t.Parallel()
	fe := NewFakeEvents()
	fe.Emit("start")
	fe.Close()

	fd := NewFakeDocker().StubEvents(fe)
	evCh, errCh := fd.Events(context.Background())

	if fd.EventsCalls() != 1 {
		t.Errorf("EventsCalls = %d, want 1", fd.EventsCalls())
	}
	_ = errCh
	var got []string
	for ev := range evCh {
		got = append(got, ev.Action)
	}
	if !slices.Equal(got, []string{"start"}) {
		t.Errorf("events = %v, want [start]", got)
	}
}

func TestFakeDockerMultipleInspectCalls(t *testing.T) {
	t.Parallel()
	fd := NewFakeDocker().
		StubInspect(Container{Running: false}, nil).
		StubInspect(Container{Running: true, Env: map[string]string{"MIRABILIS_VERSION": "v1"}}, nil)

	ctx := context.Background()
	c1, err := fd.Inspect(ctx)
	if err != nil || c1.Running {
		t.Fatalf("first inspect: running=%v err=%v, want running=false nil", c1.Running, err)
	}
	if fd.InspectCalls() != 1 {
		t.Errorf("InspectCalls after first = %d, want 1", fd.InspectCalls())
	}
	c2, err := fd.Inspect(ctx)
	if err != nil || !c2.Running {
		t.Fatalf("second inspect: running=%v err=%v, want running=true nil", c2.Running, err)
	}
	if fd.InspectCalls() != 2 {
		t.Errorf("InspectCalls after second = %d, want 2", fd.InspectCalls())
	}
}

func TestFakeDockerNoInspectStub(t *testing.T) {
	t.Parallel()
	fd := NewFakeDocker()
	_, err := fd.Inspect(context.Background())
	if err == nil {
		t.Fatal("Inspect with no stub = nil, want error")
	}
}

func TestFakeDockerEventsNoStub(t *testing.T) {
	t.Parallel()
	fd := NewFakeDocker()
	evCh, _ := fd.Events(context.Background())
	if fd.EventsCalls() != 1 {
		t.Errorf("EventsCalls = %d, want 1", fd.EventsCalls())
	}
	if evCh == nil {
		t.Fatal("Events with no stub returned nil channel")
	}
}

func TestFakeDockerLastInspectStubRepeats(t *testing.T) {
	t.Parallel()
	fd := NewFakeDocker().StubInspect(Container{Running: true}, nil)
	ctx := context.Background()

	c1, err1 := fd.Inspect(ctx)
	c2, err2 := fd.Inspect(ctx)

	if err1 != nil || !c1.Running {
		t.Fatalf("first inspect: running=%v err=%v", c1.Running, err1)
	}
	if err2 != nil || !c2.Running {
		t.Fatalf("second inspect (last stub repeats): running=%v err=%v", c2.Running, err2)
	}
}

func TestFakeDockerMultipleEventsStubs(t *testing.T) {
	t.Parallel()
	fe1 := NewFakeEvents()
	fe1.Emit("start")
	fe1.Close()

	fe2 := NewFakeEvents()
	fe2.Emit("die")
	fe2.Close()

	fd := NewFakeDocker().StubEvents(fe1).StubEvents(fe2)

	ctx := context.Background()
	evCh1, _ := fd.Events(ctx)
	evCh2, _ := fd.Events(ctx)

	if fd.EventsCalls() != 2 {
		t.Errorf("EventsCalls = %d, want 2", fd.EventsCalls())
	}

	var got1 []string
	for ev := range evCh1 {
		got1 = append(got1, ev.Action)
	}
	if !slices.Equal(got1, []string{"start"}) {
		t.Errorf("first events = %v, want [start]", got1)
	}

	var got2 []string
	for ev := range evCh2 {
		got2 = append(got2, ev.Action)
	}
	if !slices.Equal(got2, []string{"die"}) {
		t.Errorf("second events = %v, want [die]", got2)
	}
}
