//go:build integration

package provision

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestMain(m *testing.M) {
	v := m.Run()
	snaps.Clean(m)
	os.Exit(v)
}

func TestEnsureAptPackages_Integration(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx := t.Context()
	ctr, err := testcontainers.Run(ctx, "debian:bookworm-slim",
		testcontainers.WithCmd("sleep", "infinity"),
	)
	testcontainers.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start container: %v", err)
	}

	execInCtr := func(name string, args []string) (string, error) {
		code, r, err := ctr.Exec(ctx, append([]string{name}, args...), tcexec.Multiplexed())
		if err != nil {
			return "", err
		}
		out, _ := io.ReadAll(r)
		if code != 0 {
			return "", fmt.Errorf("exit %d: %s", code, strings.TrimSpace(string(out)))
		}
		return strings.TrimSpace(string(out)), nil
	}

	if _, err := execInCtr("apt-get", []string{"update"}); err != nil {
		t.Fatalf("apt-get update: %v", err)
	}
	if _, err := execInCtr("apt-get", []string{"install", "-y", "--no-install-recommends", "sudo"}); err != nil {
		t.Fatalf("install sudo: %v", err)
	}

	cfg := config.New(t.TempDir())
	if err := os.WriteFile(cfg.AptPackagesTxt(), []byte("tree\nbash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	recording := func(transcript *[]string) *runner.FakeRunner {
		return &runner.FakeRunner{
			HostFunc: func(name string, args []string) (string, error) {
				entry := name
				if len(args) > 0 {
					entry += " " + strings.Join(args, " ")
				}
				*transcript = append(*transcript, entry)
				return execInCtr(name, args)
			},
		}
	}

	var transcript1 []string
	if err := EnsureAptPackages(ctx, recording(&transcript1), cfg); err != nil {
		t.Fatalf("EnsureAptPackages (first run): %v", err)
	}

	code, _, err := ctr.Exec(ctx, []string{"dpkg", "-s", "tree"}, tcexec.Multiplexed())
	if err != nil {
		t.Fatalf("dpkg -s tree: %v", err)
	}
	if code != 0 {
		t.Error("tree not installed after EnsureAptPackages")
	}

	snaps.MatchSnapshot(t, transcript1)

	var transcript2 []string
	if err := EnsureAptPackages(ctx, recording(&transcript2), cfg); err != nil {
		t.Fatalf("EnsureAptPackages (second run): %v", err)
	}

	snaps.MatchSnapshot(t, transcript2)
}
