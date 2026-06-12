package sandbox

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

const eventBuffer = 16

type Moby struct {
	cli *client.Client
}

var _ Docker = (*Moby)(nil)

func NewMoby() (*Moby, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("sandbox: docker client: %w", err)
	}
	return &Moby{cli: cli}, nil
}

func (m *Moby) Inspect(ctx context.Context) (Container, error) {
	list, err := m.cli.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("name", ContainerName),
	})
	if err != nil {
		return Container{}, err
	}
	found := false
	for _, it := range list.Items {
		if slices.Contains(it.Names, "/"+ContainerName) {
			found = true
			break
		}
	}
	if !found {
		return Container{}, nil
	}
	res, err := m.cli.ContainerInspect(ctx, ContainerName, client.ContainerInspectOptions{})
	if err != nil {
		return Container{}, err
	}
	var out Container
	if st := res.Container.State; st != nil {
		out.Running = st.Running
		if st.Health != nil {
			out.Health = string(st.Health.Status)
		}
		if t, perr := time.Parse(time.RFC3339Nano, st.StartedAt); perr == nil {
			out.StartedAt = t
		}
	}
	if cfg := res.Container.Config; cfg != nil {
		out.Env = make(map[string]string, len(cfg.Env))
		for _, kv := range cfg.Env {
			if k, v, ok := strings.Cut(kv, "="); ok {
				out.Env[k] = v
			}
		}
	}
	for _, mnt := range res.Container.Mounts {
		out.Mounts = append(out.Mounts, mnt.Destination)
	}
	return out, nil
}

func (m *Moby) Events(ctx context.Context) (<-chan ContainerEvent, <-chan error) {
	res := m.cli.Events(ctx, client.EventsListOptions{
		Filters: make(client.Filters).
			Add("type", "container").
			Add("container", ContainerName),
	})
	out := make(chan ContainerEvent, eventBuffer)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-res.Err:
				if err != nil {
					errs <- err
				}
				return
			case msg := <-res.Messages:
				select {
				case out <- ContainerEvent{Action: string(msg.Action)}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, errs
}
