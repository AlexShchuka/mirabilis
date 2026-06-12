//go:build integration

package sandbox

import (
	"context"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func requireDaemon(t *testing.T) *client.Client {
	t.Helper()
	cli, err := client.New(client.FromEnv)
	if err != nil {
		t.Skipf("docker daemon unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := cli.Ping(ctx, client.PingOptions{}); err != nil {
		t.Skipf("docker daemon unavailable: %v", err)
	}
	return cli
}

func existingMirabilis(t *testing.T, cli *client.Client) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cli.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("name", ContainerName),
	})
	if err != nil {
		t.Fatalf("ContainerList: %v", err)
	}
	for _, it := range res.Items {
		for _, n := range it.Names {
			if n == "/"+ContainerName {
				return true
			}
		}
	}
	return false
}

func pullAlpine(t *testing.T, cli *client.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := cli.ImagePull(ctx, "alpine:latest", client.ImagePullOptions{})
	if err != nil {
		t.Fatalf("ImagePull alpine: %v", err)
	}
	if err := resp.Wait(ctx); err != nil {
		t.Fatalf("ImagePull wait: %v", err)
	}
}

func createMirabilis(t *testing.T, cli *client.Client) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: "alpine:latest",
			Cmd:   []string{"sleep", "120"},
		},
		Name: ContainerName,
	})
	if err != nil {
		t.Fatalf("ContainerCreate: %v", err)
	}
	return res.ID
}

func startMirabilis(t *testing.T, cli *client.Client, id string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := cli.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		t.Fatalf("ContainerStart: %v", err)
	}
}

func removeMirabilis(t *testing.T, cli *client.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := cli.ContainerRemove(ctx, ContainerName, client.ContainerRemoveOptions{Force: true}); err != nil {
		t.Logf("ContainerRemove (cleanup): %v", err)
	}
}

func TestMobyInspectIntegration(t *testing.T) {
	cli := requireDaemon(t)
	m, err := NewMoby()
	if err != nil {
		t.Fatalf("NewMoby: %v", err)
	}

	if existingMirabilis(t, cli) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		c, inspErr := m.Inspect(ctx)
		if inspErr != nil {
			t.Fatalf("Inspect (existing container): %v", inspErr)
		}
		if !c.Running {
			t.Skip("mirabilis container exists but is not running; skipping running-state assertion")
		}
		if c.StartedAt.IsZero() {
			t.Error("Inspect: StartedAt is zero for a running container")
		}
		return
	}

	pullAlpine(t, cli)
	id := createMirabilis(t, cli)
	t.Cleanup(func() { removeMirabilis(t, cli) })

	startMirabilis(t, cli, id)

	deadline := time.Now().Add(10 * time.Second)
	var c Container
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		c, err = m.Inspect(ctx)
		cancel()
		if err != nil {
			t.Fatalf("Inspect after start: %v", err)
		}
		if c.Running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("container did not reach Running=true within deadline")
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !c.Running {
		t.Error("Inspect: Running = false, want true")
	}

	if _, removeErr := cli.ContainerRemove(context.Background(), ContainerName, client.ContainerRemoveOptions{Force: true}); removeErr != nil {
		t.Fatalf("ContainerRemove before not-found check: %v", removeErr)
	}

	notFoundDeadline := time.Now().Add(5 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		c, err = m.Inspect(ctx)
		cancel()
		if err != nil {
			t.Fatalf("Inspect after remove: %v", err)
		}
		if !c.Running {
			break
		}
		if time.Now().After(notFoundDeadline) {
			t.Fatal("container still visible as running after forced removal within deadline")
		}
		time.Sleep(100 * time.Millisecond)
	}

	if c.Running {
		t.Errorf("Inspect after remove: Running = true, want false")
	}
	if c.Health != "" {
		t.Errorf("Inspect after remove: Health = %q, want empty", c.Health)
	}
	if !c.StartedAt.IsZero() {
		t.Errorf("Inspect after remove: StartedAt = %v, want zero", c.StartedAt)
	}
	if len(c.Env) != 0 {
		t.Errorf("Inspect after remove: Env = %v, want empty", c.Env)
	}
	if len(c.Mounts) != 0 {
		t.Errorf("Inspect after remove: Mounts = %v, want empty", c.Mounts)
	}
}

func smokeEventsSubscription(t *testing.T, m *Moby) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	events, errs := m.Events(ctx)
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	eventsOpen := true
	errsOpen := true
	for eventsOpen || errsOpen {
		select {
		case _, ok := <-events:
			if !ok {
				eventsOpen = false
				events = nil
			}
		case e, ok := <-errs:
			if !ok {
				errsOpen = false
				errs = nil
				continue
			}
			if e != nil {
				t.Fatalf("Events smoke: unexpected error: %v", e)
			}
		case <-time.After(time.Until(deadline)):
			t.Fatal("Events channels did not close within deadline after context cancel")
		}
	}
}

func TestMobyEventsIntegration(t *testing.T) {
	cli := requireDaemon(t)
	m, err := NewMoby()
	if err != nil {
		t.Fatalf("NewMoby: %v", err)
	}

	if existingMirabilis(t, cli) {
		smokeEventsSubscription(t, m)
		return
	}

	pullAlpine(t, cli)
	id := createMirabilis(t, cli)
	t.Cleanup(func() { removeMirabilis(t, cli) })

	eventsCtx, eventsCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer eventsCancel()
	events, errs := m.Events(eventsCtx)

	startMirabilis(t, cli, id)

	startDeadline := time.Now().Add(10 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		c, inspErr := m.Inspect(ctx)
		cancel()
		if inspErr != nil {
			t.Fatalf("Inspect waiting for start: %v", inspErr)
		}
		if c.Running {
			break
		}
		if time.Now().After(startDeadline) {
			t.Fatal("container did not reach Running=true within deadline before pause")
		}
		time.Sleep(100 * time.Millisecond)
	}

	pauseCtx, pauseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, pauseErr := cli.ContainerPause(pauseCtx, id, client.ContainerPauseOptions{}); pauseErr != nil {
		pauseCancel()
		t.Fatalf("ContainerPause: %v", pauseErr)
	}
	pauseCancel()

	unpauseCtx, unpauseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, unpauseErr := cli.ContainerUnpause(unpauseCtx, id, client.ContainerUnpauseOptions{}); unpauseErr != nil {
		unpauseCancel()
		t.Fatalf("ContainerUnpause: %v", unpauseErr)
	}
	unpauseCancel()

	var received []string
	eventDeadline := time.After(15 * time.Second)
	for len(received) < 2 {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("events channel closed unexpectedly; got %v so far", received)
			}
			received = append(received, ev.Action)
		case e := <-errs:
			if e != nil {
				t.Fatalf("Events error: %v", e)
			}
		case <-eventDeadline:
			t.Fatalf("did not receive 2 events within deadline; got %v", received)
		}
	}

	foundPause := false
	foundUnpause := false
	for _, a := range received {
		if a == "pause" {
			foundPause = true
		}
		if a == "unpause" {
			foundUnpause = true
		}
	}
	if !foundPause {
		t.Errorf("Events: no pause action; got %v", received)
	}
	if !foundUnpause {
		t.Errorf("Events: no unpause action; got %v", received)
	}

	eventsCancel()

	closeDeadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				goto eventsDone
			}
		case e, ok := <-errs:
			if !ok {
				goto eventsDone
			}
			if e != nil {
				t.Fatalf("Events drain: unexpected error: %v", e)
			}
		case <-time.After(time.Until(closeDeadline)):
			t.Fatal("Events channels did not close within deadline after context cancel")
		}
	}
eventsDone:
}
