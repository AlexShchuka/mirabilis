package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func EnsureGitIdentity(ctx context.Context, r runner.Runner) error {
	if _, err := r.Host(ctx, "gh", "auth", "status"); err != nil {
		return nil
	}
	if _, err := r.Host(ctx, "git", "version"); err != nil {
		return nil
	}

	raw, err := r.Host(ctx, "gh", "api", "user")
	if err != nil || raw == "" {
		return nil
	}

	var user struct {
		Login string      `json:"login"`
		Name  string      `json:"name"`
		Email string      `json:"email"`
		ID    json.Number `json:"id"`
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	if err := dec.Decode(&user); err != nil || user.Login == "" {
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

	if _, err := r.Host(ctx, "git", "config", "--global", "user.name", name); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: git config user.name: %v\n", err)
	}
	if _, err := r.Host(ctx, "git", "config", "--global", "user.email", email); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: git config user.email: %v\n", err)
	}

	if _, err := r.Host(ctx, "gh", "auth", "setup-git"); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: gh auth setup-git: %v\n", err)
	}
	return nil
}
