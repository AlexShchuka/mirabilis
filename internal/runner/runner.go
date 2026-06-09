package runner

import "context"

type Runner interface {
	Host(ctx context.Context, name string, args ...string) (string, error)
	Container(ctx context.Context, args ...string) (string, error)
	Repo() string
}
