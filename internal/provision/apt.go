package provision

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/config"
	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func EnsureAptPackages(ctx context.Context, r runner.Runner, cfg config.Config) error {
	f, err := os.Open(cfg.AptPackagesTxt())
	if err != nil {
		return nil
	}
	defer f.Close()

	var missing []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		pkg := strings.TrimSpace(sc.Text())
		if pkg == "" {
			continue
		}
		if _, err := r.Host(ctx, "dpkg", "-s", pkg); err != nil {
			missing = append(missing, pkg)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	if _, err := r.Host(ctx, "sudo", "apt-get", "update"); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: apt-get update: %v\n", err)
		return nil
	}
	args := append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, missing...)
	if _, err := r.Host(ctx, "sudo", args...); err != nil {
		fmt.Fprintf(os.Stderr, "[provision] WARN: declared apt packages not fully applied: %s: %v\n", strings.Join(missing, " "), err)
	}
	return nil
}
