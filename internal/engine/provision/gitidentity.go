package provision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type gitIdentityStep struct {
	d Deps
}

func (s *gitIdentityStep) Meta() pipeline.Meta { return carryMeta("git-identity", "Git identity") }

func (s *gitIdentityStep) Check(ctx context.Context) (bool, error) {
	if !s.d.cmd().argvOK(ctx, "gh", "auth", "status") {
		return true, nil
	}
	if !s.d.cmd().argvOK(ctx, "git", "version") {
		return true, nil
	}
	name, err := s.d.cmd().output(ctx, "git", "config", "--global", "user.name")
	if err != nil || strings.TrimSpace(name) == "" {
		return false, nil
	}
	email, err := s.d.cmd().output(ctx, "git", "config", "--global", "user.email")
	if err != nil || strings.TrimSpace(email) == "" {
		return false, nil
	}
	return true, nil
}

func (s *gitIdentityStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	if !s.d.cmd().argvOK(ctx, "gh", "auth", "status") {
		return nil
	}
	if !s.d.cmd().argvOK(ctx, "git", "version") {
		return nil
	}
	raw, err := s.d.cmd().output(ctx, "gh", "api", "user")
	if err != nil || raw == "" {
		return nil
	}
	var user struct {
		Login string      `json:"login"`
		Name  string      `json:"name"`
		Email string      `json:"email"`
		ID    json.Number `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &user); err != nil || user.Login == "" {
		return nil
	}
	name := user.Name
	if name == "" {
		name = user.Login
	}
	email := user.Email
	if email == "" {
		email = fmt.Sprintf("%s+%s@users.noreply.github.com", user.ID.String(), user.Login)
	}
	var errs []error
	if err := s.d.cmd().stream(ctx, "git-identity", out, "git", "config", "--global", "user.name", name); err != nil {
		errs = append(errs, fmt.Errorf("git config user.name: %w", err))
	}
	if err := s.d.cmd().stream(ctx, "git-identity", out, "git", "config", "--global", "user.email", email); err != nil {
		errs = append(errs, fmt.Errorf("git config user.email: %w", err))
	}
	if err := s.d.cmd().stream(ctx, "git-identity", out, "gh", "auth", "setup-git"); err != nil {
		errs = append(errs, fmt.Errorf("gh auth setup-git: %w", err))
	}
	return errors.Join(errs...)
}
