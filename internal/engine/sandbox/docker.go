package sandbox

import (
	"context"
	"time"
)

const ContainerName = "mirabilis"

type Container struct {
	Env       map[string]string
	Health    string
	Mounts    []string
	StartedAt time.Time
	Running   bool
}

type ContainerEvent struct {
	Action string
}

type Docker interface {
	Inspect(ctx context.Context) (Container, error)
	Events(ctx context.Context) (<-chan ContainerEvent, <-chan error)
}
