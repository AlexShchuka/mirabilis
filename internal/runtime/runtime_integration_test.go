//go:build integration

package runtime

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

func TestSecretReachesContainerNotArgv(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx := t.Context()
	ctr, err := testcontainers.Run(ctx, "debian:bookworm-slim",
		testcontainers.WithCmd("sleep", "infinity"),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	ctrID := ctr.GetContainerID()

	t.Run("ComposeEnv blocks the bot token from the container", func(t *testing.T) {
		// Host-outbox model (#115): the bot token is held only on the host and
		// must NEVER reach the container. Even with TELEGRAM_BOT_TOKEN set in the
		// host environment, ComposeEnv must strip it — verified here at the
		// integration level (the unit-level guarantee is TestComposeEnv_TokenNeverInjected).
		const sentinel1 = "mirabilis-it-telegram-x"
		t.Setenv("TELEGRAM_BOT_TOKEN", sentinel1)

		repo := makeGitRepo(t)
		for _, kv := range ComposeEnv(repo) {
			if strings.HasPrefix(kv, "TELEGRAM_BOT_TOKEN=") {
				t.Errorf("TELEGRAM_BOT_TOKEN must not be passed to the container, got %q", kv)
			}
			if strings.Contains(kv, sentinel1) {
				t.Errorf("token sentinel leaked into ComposeEnv: %q", kv)
			}
		}
	})

	t.Run("handoff token path", func(t *testing.T) {
		ctx := t.Context()
		const sentinel2 = "mirabilis-it-ghpat-x"
		repo := makeGitRepo(t)
		env := handoffEnv(ComposeEnv(repo), sentinel2)

		cmd := exec.CommandContext(ctx, "docker", "exec",
			"-e", "GITHUB_PERSONAL_ACCESS_TOKEN",
			ctrID,
			"printenv", "GITHUB_PERSONAL_ACCESS_TOKEN",
		)
		cmd.Env = env

		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("docker exec printenv: %v", err)
		}
		got := strings.TrimSpace(string(out))
		if got != sentinel2 {
			t.Errorf("GITHUB_PERSONAL_ACCESS_TOKEN in container = %q, want %q", got, sentinel2)
		}
		for _, arg := range cmd.Args {
			if strings.Contains(arg, sentinel2) {
				t.Errorf("sentinel2 found in argv: %q", arg)
			}
		}
	})
}
