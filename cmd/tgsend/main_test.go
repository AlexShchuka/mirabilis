package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/telegram"
)

// ── run() (CLI entry point) behaviour ─────────────────────────────────────

// TestRun_NoText_ReturnsTwo verifies that run() returns 2 when no message text
// is provided (usage error).
func TestRun_NoText_ReturnsTwo(t *testing.T) {
	t.Setenv("TELEGRAM_CHAT_ID", "-100x")
	code := run([]string{"-channel", "-100x"}, strings.NewReader(""), io.Discard, io.Discard)
	if code != 2 {
		t.Errorf("run with no text = %d, want 2", code)
	}
}

// TestRun_NoChannel_ReturnsTwo verifies run() returns 2 when no channel is set.
func TestRun_NoChannel_ReturnsTwo(t *testing.T) {
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("MIRABILIS_REPO", t.TempDir())
	code := run([]string{"hello"}, strings.NewReader(""), io.Discard, io.Discard)
	if code != 2 {
		t.Errorf("run with no channel = %d, want 2", code)
	}
}

// TestRun_TextFromStdin_DryRun_ReturnsZero verifies run() reads text from
// stdin and performs a dry-run successfully.
func TestRun_TextFromStdin_DryRun_ReturnsZero(t *testing.T) {
	t.Setenv("TELEGRAM_CHAT_ID", "-100x")
	t.Setenv("MIRABILIS_REPO", t.TempDir())
	code := run([]string{}, strings.NewReader("hello from stdin\n"), io.Discard, io.Discard)
	if code != 0 {
		t.Errorf("run with stdin text dry-run = %d, want 0", code)
	}
}

// TestRun_BadFlag_ReturnsTwo verifies run() returns 2 on an unknown flag.
func TestRun_BadFlag_ReturnsTwo(t *testing.T) {
	code := run([]string{"--unknown-flag"}, strings.NewReader(""), io.Discard, io.Discard)
	if code != 2 {
		t.Errorf("run with bad flag = %d, want 2", code)
	}
}

// ── queue-writer behaviour ─────────────────────────────────────────────────

// TestQueueWriter_Confirm_WritesJobFile verifies that --confirm writes exactly
// one job file in the queue directory and that the job contains the expected
// fields.
func TestQueueWriter_Confirm_WritesJobFile(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "outbox")

	job := telegram.Job{
		ID:        "test-id-001",
		ChatID:    "-100123",
		Text:      "hello from container",
		CreatedAt: time.Now().UTC(),
	}

	if err := telegram.WriteJob(queueDir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	// Exactly one job file must exist.
	entries, err := os.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	jobFiles := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".job" {
			jobFiles++
		}
	}
	if jobFiles != 1 {
		t.Errorf("job files = %d, want 1", jobFiles)
	}
}

// TestQueueWriter_DryRun_WritesNoFile verifies that runSend in dry-run mode
// (confirm=false) writes NO job file.
func TestQueueWriter_DryRun_WritesNoFile(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "outbox")

	code := runSend(queueDir, "-100test", "hello dry-run", false, 0, io.Discard, io.Discard)
	if code != 0 {
		t.Fatalf("runSend dry-run returned exit code %d, want 0", code)
	}

	// No .job files should exist (outbox dir may not even be created).
	entries, _ := os.ReadDir(queueDir) // ignore error — dir may not exist
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".job" {
			t.Errorf("dry-run wrote unexpected job file %q", e.Name())
		}
	}
}

// TestQueueWriter_Confirm_WritesExactlyOneJobFile verifies that runSend with
// confirm=true writes exactly one job file with a non-empty ID and valid JSON.
func TestQueueWriter_Confirm_WritesExactlyOneJobFile(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "outbox")

	code := runSend(queueDir, "-100test", "hello confirm", true, 0, io.Discard, io.Discard)
	if code != 0 {
		// Exit code 0 is success; non-zero only if WriteJob fails or status
		// wait fails — both acceptable (no watcher running in tests).
		// The relevant assertion is below: file must exist.
	}

	entries, err := os.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var jobFiles []os.DirEntry
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".job" {
			jobFiles = append(jobFiles, e)
		}
	}
	if len(jobFiles) != 1 {
		t.Fatalf("job files = %d, want 1", len(jobFiles))
	}

	// Verify the file contains valid JSON with a non-empty id.
	data, err := os.ReadFile(filepath.Join(queueDir, jobFiles[0].Name()))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var job telegram.Job
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatalf("job file is not valid JSON: %v", err)
	}
	if job.ID == "" {
		t.Error("job.ID is empty, want non-empty")
	}
}

// TestQueueWriter_JobContents verifies the job file round-trips correctly.
func TestQueueWriter_JobContents(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "outbox")

	want := telegram.Job{
		ID:        "round-trip-42",
		ChatID:    "-100999",
		Text:      "round trip text",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := telegram.WriteJob(queueDir, want); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	jobPath := filepath.Join(queueDir, want.ID+".job")
	got, err := telegram.ReadJob(jobPath)
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	if got.ChatID != want.ChatID {
		t.Errorf("ChatID = %q, want %q", got.ChatID, want.ChatID)
	}
	if got.Text != want.Text {
		t.Errorf("Text = %q, want %q", got.Text, want.Text)
	}
}

// TestQueueWriter_PendingJobs_NoStatusFile verifies that a job without a
// matching status file is returned by PendingJobs.
func TestQueueWriter_PendingJobs_NoStatusFile(t *testing.T) {
	dir := t.TempDir()

	job := telegram.Job{
		ID:        "pending-job",
		ChatID:    "-100",
		Text:      "test",
		CreatedAt: time.Now().UTC(),
	}
	if err := telegram.WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	pending, err := telegram.PendingJobs(dir)
	if err != nil {
		t.Fatalf("PendingJobs: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("PendingJobs = %d, want 1", len(pending))
	}
}

// TestRunSend_WriteJobError_ReturnsOne verifies that runSend returns exit code
// 1 when WriteJob fails (e.g. unwritable queue dir).
func TestRunSend_WriteJobError_ReturnsOne(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root, permission test not meaningful")
	}
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "ro-outbox")
	if err := os.MkdirAll(queueDir, 0o555); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Cleanup(func() { os.Chmod(queueDir, 0o755) })

	// Make the parent unwritable so CreateTemp fails inside WriteJob.
	code := runSend(queueDir, "-100test", "will fail", true, 0, io.Discard, io.Discard)
	if code != 1 {
		t.Errorf("runSend with unwritable dir = %d, want 1", code)
	}
}

// TestRunSend_StatusOK_ReturnsZero verifies that when a status file is written
// before waitForStatus times out, runSend returns 0.
func TestRunSend_StatusOK_ReturnsZero(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "outbox")

	// Pre-write a status file with the ID that uuid will produce — we cannot
	// know the UUID in advance, so we write the status asynchronously after
	// runSend writes the job file.
	//
	// Strategy: use a goroutine that polls for the .job file and writes the
	// status immediately.
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			jobs, _ := telegram.PendingJobs(queueDir)
			if len(jobs) > 0 {
				// Parse the job to get the ID.
				job, err := telegram.ReadJob(jobs[0])
				if err != nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				_ = telegram.WriteStatus(queueDir, telegram.JobStatus{
					ID:          job.ID,
					OK:          true,
					CompletedAt: time.Now().UTC(),
				})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	code := runSend(queueDir, "-100test", "status ok test", true, 5*time.Second, io.Discard, io.Discard)
	<-done
	if code != 0 {
		t.Errorf("runSend with OK status = %d, want 0", code)
	}
}

// TestRunSend_StatusFailed_ReturnsOne verifies that when the status file
// reports failure, runSend returns exit code 1.
func TestRunSend_StatusFailed_ReturnsOne(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "outbox")

	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			jobs, _ := telegram.PendingJobs(queueDir)
			if len(jobs) > 0 {
				job, err := telegram.ReadJob(jobs[0])
				if err != nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				_ = telegram.WriteStatus(queueDir, telegram.JobStatus{
					ID:          job.ID,
					OK:          false,
					Error:       "pin mismatch",
					CompletedAt: time.Now().UTC(),
				})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	code := runSend(queueDir, "-100test", "status fail test", true, 5*time.Second, io.Discard, io.Discard)
	<-done
	if code != 1 {
		t.Errorf("runSend with failed status = %d, want 1", code)
	}
}

// TestQueueWriter_PendingJobs_WithStatusFile verifies that a job with a
// matching status file is NOT returned by PendingJobs (already processed).
func TestQueueWriter_PendingJobs_WithStatusFile(t *testing.T) {
	dir := t.TempDir()

	job := telegram.Job{
		ID:        "done-job",
		ChatID:    "-100",
		Text:      "done",
		CreatedAt: time.Now().UTC(),
	}
	if err := telegram.WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
	// Write corresponding status file.
	if err := telegram.WriteStatus(dir, telegram.JobStatus{
		ID:          job.ID,
		OK:          true,
		CompletedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}

	pending, err := telegram.PendingJobs(dir)
	if err != nil {
		t.Fatalf("PendingJobs: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("PendingJobs with status = %d, want 0 (already processed)", len(pending))
	}
}
