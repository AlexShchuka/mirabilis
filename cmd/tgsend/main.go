// tgsend queues a Telegram message for the host-side watcher to deliver.
//
// tgsend runs INSIDE the devcontainer. It holds NO token and makes NO HTTP
// calls to Telegram. Instead it writes a job file into the shared queue
// directory (<repo>/.mirabilis/outbox/) that is bind-mounted into the
// container. The host-side `mirabilis tg-outbox` watcher picks up the job
// and delivers it through internal/telegram.NewOutbox (which enforces the
// channel pin and rate limit).
//
// Channel: supplied via -channel flag; defaults to TELEGRAM_CHAT_ID env var
// injected by ComposeEnv (not a secret — set from host keychain at container
// start).
//
// Queue directory: derived from MIRABILIS_REPO env var (set by ComposeEnv)
// via internal/telegram.OutboxDir. Falls back to /workspace if not set.
//
// Dry-run (default): prints what would be queued, writes nothing.
// With --confirm: writes the job file atomically.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/telegram"
	"github.com/google/uuid"
)

// defaultRepoPath is the fallback workspace path when MIRABILIS_REPO is not set.
const defaultRepoPath = "/workspace"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is the testable entry-point for tgsend. It parses args, reads text from
// stdin if needed, resolves the channel and queue dir, and delegates to
// runSend. Returns an exit code (0 = success, 1 = error, 2 = usage error).
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tgsend", flag.ContinueOnError)
	fs.SetOutput(stderr)
	channelFlag := fs.String("channel", "", "channel id (e.g. @mychannel or -100…); defaults to TELEGRAM_CHAT_ID")
	confirm := fs.Bool("confirm", false, "write the job file into the queue (default: dry-run only)")
	wait := fs.Duration("wait", 0, "block up to this long for delivery status (0 = don't wait; job is delivered by `mirabilis tg-outbox`)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	text := fs.Arg(0)
	if text == "" {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "tgsend: read stdin: %v\n", err)
			return 1
		}
		text = strings.TrimRight(string(raw), "\r\n")
	}
	if text == "" {
		fmt.Fprintln(stderr, "tgsend: message text is required (positional arg or stdin)")
		return 2
	}

	// Resolve channel (not a secret; injected by ComposeEnv from host keychain).
	channel := *channelFlag
	if channel == "" {
		channel = os.Getenv("TELEGRAM_CHAT_ID")
	}
	if channel == "" {
		fmt.Fprintln(stderr, "tgsend: no channel available — supply -channel <id> or run the provision step")
		return 2
	}

	// Resolve queue directory from repo root (bind-mounted workspace).
	repo := os.Getenv("MIRABILIS_REPO")
	if repo == "" {
		repo = defaultRepoPath
	}
	queueDir := telegram.OutboxDir(repo)

	return runSend(queueDir, channel, text, *confirm, *wait, stdout, stderr)
}

// runSend is the testable core of tgsend. It writes a job file (if confirm is
// true) or prints a dry-run summary (if false). It returns 0 on success, 1 on
// fatal error. stdout and stderr are separated so tests can silence output.
func runSend(queueDir, channel, text string, confirm bool, statusWait time.Duration, stdout, stderr io.Writer) int {
	if !confirm {
		// Dry-run: print what would be queued, write nothing.
		fmt.Fprintf(stdout, "dry-run: would queue to channel %s\nmessage: %s\nqueue:   %s\n", channel, text, queueDir)
		fmt.Fprintln(stderr, "(pass --confirm to write the job file)")
		return 0
	}

	job := telegram.Job{
		ID:        uuid.NewString(),
		ChatID:    channel,
		Text:      text,
		CreatedAt: time.Now().UTC(),
	}
	if err := telegram.WriteJob(queueDir, job); err != nil {
		fmt.Fprintf(stderr, "tgsend: queue write failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "queued job %s to channel %s\n", job.ID, channel)

	// By default do not block: the job is on disk and the host watcher delivers it.
	if statusWait <= 0 {
		fmt.Fprintln(stderr, "tgsend: job queued (run `mirabilis tg-outbox` on the host to deliver; pass -wait to block for status)")
		return 0
	}

	// -wait given: poll for the status file for up to statusWait.
	status, err := waitForStatus(queueDir, job.ID, statusWait)
	if err != nil {
		// Watcher not running or timed out — not fatal, job is on disk.
		fmt.Fprintln(stderr, "tgsend: job queued but no status yet (is mirabilis tg-outbox running?)")
		return 0
	}
	if status.OK {
		fmt.Fprintf(stdout, "delivered: job %s ok\n", job.ID)
	} else {
		fmt.Fprintf(stderr, "tgsend: delivery failed: %s\n", status.Error)
		return 1
	}
	return 0
}

// waitForStatus polls for a status file for up to timeout.
func waitForStatus(dir, jobID string, timeout time.Duration) (telegram.JobStatus, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s, err := telegram.ReadStatus(dir, jobID)
		if err == nil {
			return s, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return telegram.JobStatus{}, fmt.Errorf("timeout waiting for status")
}
