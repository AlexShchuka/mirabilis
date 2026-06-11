package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// helpers re-used from outbox_test.go are in the same package (telegram);
// writeTokenFile and alwaysConfirm are defined there.

// newMockTelegramServer returns a test server that responds with ok:true and
// records how many times it was called plus the last chat_id it received.
func newMockTelegramServer(t *testing.T, calls *atomic.Int32, lastChatID *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if err := r.ParseForm(); err == nil {
			*lastChatID = r.FormValue("chat_id")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
}

// TestProcessOneJob_PinnedChatID_PostsAndWritesStatus is the live-path
// integration test:
//
//   - writes a job file with the PINNED chat_id
//   - calls ProcessOneJob against an httptest mock Telegram server
//   - asserts the mock was called exactly once with the correct chat_id
//   - asserts a status file was written and marks OK
func TestProcessOneJob_PinnedChatID_PostsAndWritesStatus(t *testing.T) {
	const pinnedID = "-100999001"

	var calls atomic.Int32
	var lastChatID string
	srv := newMockTelegramServer(t, &calls, &lastChatID)
	defer srv.Close()

	dir := t.TempDir()
	tokPath := writeTokenFile(t, "tok-watcher-live")

	ob, err := NewOutbox(tokPath, pinnedID, alwaysConfirm)
	if err != nil {
		t.Fatalf("NewOutbox: %v", err)
	}
	ob.withBaseURL(srv.URL)

	job := Job{
		ID:        "live-job-001",
		ChatID:    pinnedID,
		Text:      "hello from watcher test",
		CreatedAt: time.Now().UTC(),
	}
	jobPath := filepath.Join(dir, job.ID+jobSuffix)
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	ProcessOneJob(context.Background(), dir, jobPath, ob)

	// Assert: mock was called once.
	if n := calls.Load(); n != 1 {
		t.Errorf("mock called %d times, want 1", n)
	}
	// Assert: correct chat_id was posted.
	if lastChatID != pinnedID {
		t.Errorf("posted chat_id = %q, want %q", lastChatID, pinnedID)
	}
	// Assert: status file was written and is OK.
	status, err := ReadStatus(dir, job.ID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if !status.OK {
		t.Errorf("status.OK = false, want true; error = %q", status.Error)
	}
}

// TestProcessOneJob_NonPinnedChatID_RefusedBeforeNetwork verifies that a job
// with a chat_id that does not match the pin is refused without any HTTP call.
func TestProcessOneJob_NonPinnedChatID_RefusedBeforeNetwork(t *testing.T) {
	const pinnedID = "-100999001"
	const differentID = "-100888000"

	var calls atomic.Int32
	var lastChatID string
	srv := newMockTelegramServer(t, &calls, &lastChatID)
	defer srv.Close()

	dir := t.TempDir()
	tokPath := writeTokenFile(t, "tok-watcher-pin")

	ob, err := NewOutbox(tokPath, pinnedID, alwaysConfirm)
	if err != nil {
		t.Fatalf("NewOutbox: %v", err)
	}
	ob.withBaseURL(srv.URL)

	job := Job{
		ID:        "pin-mismatch-001",
		ChatID:    differentID, // NOT the pinned ID
		Text:      "should be refused",
		CreatedAt: time.Now().UTC(),
	}
	jobPath := filepath.Join(dir, job.ID+jobSuffix)
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	ProcessOneJob(context.Background(), dir, jobPath, ob)

	// Assert: NO HTTP call was made (pin check is before network).
	if n := calls.Load(); n != 0 {
		t.Errorf("mock called %d times, want 0 (pin mismatch should not reach network)", n)
	}
	// Assert: status file was written and marks failure.
	status, err := ReadStatus(dir, job.ID)
	if err != nil {
		t.Fatalf("ReadStatus for failed job: %v", err)
	}
	if status.OK {
		t.Error("status.OK = true, want false (pin mismatch must fail)")
	}
	if status.Error == "" {
		t.Error("status.Error is empty, want a non-empty error message")
	}
}

// TestQueueIO_WriteAndReadJob is a basic round-trip test for the queue
// serialisation.
func TestQueueIO_WriteAndReadJob(t *testing.T) {
	dir := t.TempDir()
	want := Job{
		ID:        "qio-1",
		ChatID:    "-100",
		Text:      "queue io test",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := WriteJob(dir, want); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
	got, err := ReadJob(filepath.Join(dir, want.ID+jobSuffix))
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}
	if got.ID != want.ID || got.ChatID != want.ChatID || got.Text != want.Text {
		t.Errorf("ReadJob = %+v, want %+v", got, want)
	}
}

// TestProcessQueue_EmptyDir_NoError verifies that processQueue on an empty dir
// does not return an error.
func TestProcessQueue_EmptyDir_NoError(t *testing.T) {
	dir := t.TempDir()
	// Create outbox sub-dir.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	tokPath := writeTokenFile(t, "tok-empty")
	ob, err := NewOutbox(tokPath, "-100", alwaysConfirm)
	if err != nil {
		t.Fatalf("NewOutbox: %v", err)
	}
	if err := processQueue(context.Background(), dir, ob); err != nil {
		t.Errorf("processQueue on empty dir = %v, want nil", err)
	}
}

// TestOutboxDir_ReturnsCorrectPath verifies OutboxDir constructs the expected
// path.
func TestOutboxDir_ReturnsCorrectPath(t *testing.T) {
	got := OutboxDir("/home/user/project")
	want := "/home/user/project/" + OutboxSubdir
	if got != want {
		t.Errorf("OutboxDir = %q, want %q", got, want)
	}
}

// TestReadStatus_MissingFile_Error verifies ReadStatus returns an error when
// no status file exists.
func TestReadStatus_MissingFile_Error(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadStatus(dir, "nonexistent-id")
	if err == nil {
		t.Error("ReadStatus for missing file = nil, want error")
	}
}

// TestReadStatus_BadJSON_Error verifies ReadStatus returns an error when the
// file contains invalid JSON.
func TestReadStatus_BadJSON_Error(t *testing.T) {
	dir := t.TempDir()
	// Write a corrupt status file.
	if err := os.WriteFile(filepath.Join(dir, "bad-id.status"), []byte("{invalid}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadStatus(dir, "bad-id")
	if err == nil {
		t.Error("ReadStatus for bad JSON = nil, want error")
	}
}

// TestAlwaysConfirmWatcher_AlwaysTrue exercises alwaysConfirmWatcher via
// processOneJob (the only caller in production code), so coverage fires on
// the confirm callback.
func TestAlwaysConfirmWatcher_AlwaysTrue(t *testing.T) {
	const pinnedID = "-100777001"

	var calls atomic.Int32
	var lastChatID string
	srv := newMockTelegramServer(t, &calls, &lastChatID)
	defer srv.Close()

	dir := t.TempDir()
	tokPath := writeTokenFile(t, "tok-watcher-confirm")

	// Use alwaysConfirmWatcher directly to verify it returns true.
	if got := alwaysConfirmWatcher("any-chat", "any-text"); !got {
		t.Error("alwaysConfirmWatcher = false, want true")
	}

	// Also exercise it via processOneJob (production code path).
	ob, err := NewOutbox(tokPath, pinnedID, alwaysConfirmWatcher)
	if err != nil {
		t.Fatalf("NewOutbox: %v", err)
	}
	ob.withBaseURL(srv.URL)

	job := Job{
		ID:        "confirm-watcher-001",
		ChatID:    pinnedID,
		Text:      "test confirm via watcher",
		CreatedAt: time.Now().UTC(),
	}
	jobPath := filepath.Join(dir, job.ID+jobSuffix)
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
	ProcessOneJob(context.Background(), dir, jobPath, ob)
	if calls.Load() == 0 {
		t.Error("expected HTTP call via alwaysConfirmWatcher path")
	}
}

// TestProcessOneJob_BadJobFile_NoStatus verifies that a corrupt job file is
// skipped without panicking and no status file is written for it.
func TestProcessOneJob_BadJobFile_NoStatus(t *testing.T) {
	dir := t.TempDir()
	tokPath := writeTokenFile(t, "tok-bad")
	ob, err := NewOutbox(tokPath, "-100", alwaysConfirm)
	if err != nil {
		t.Fatalf("NewOutbox: %v", err)
	}

	// Write a corrupt job file.
	badPath := filepath.Join(dir, "corrupt-id.job")
	if err := os.WriteFile(badPath, []byte("{not-valid-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Must not panic; status file must NOT be written.
	ProcessOneJob(context.Background(), dir, badPath, ob)

	if _, err := ReadStatus(dir, "corrupt-id"); err == nil {
		t.Error("status file should NOT be written for corrupt job")
	}
}

// TestRunWatcher_CancelledImmediately verifies RunWatcher exits cleanly when
// context is cancelled before the first tick.
func TestRunWatcher_CancelledImmediately(t *testing.T) {
	dir := t.TempDir()
	tokPath := writeTokenFile(t, "tok-runwatcher")

	cfg := WatcherConfig{
		QueueDir:      dir,
		TokenPath:     tokPath,
		AllowedChatID: "-100",
		PollInterval:  100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	if err := RunWatcher(ctx, cfg); err != nil {
		t.Errorf("RunWatcher cancelled = %v, want nil", err)
	}
}

// TestRunWatcher_MissingQueueDir_Error verifies RunWatcher returns error on
// empty QueueDir.
func TestRunWatcher_MissingQueueDir_Error(t *testing.T) {
	err := RunWatcher(context.Background(), WatcherConfig{
		QueueDir:  "",
		TokenPath: "/some/path",
	})
	if err == nil {
		t.Error("RunWatcher with empty QueueDir = nil, want error")
	}
}

// TestRunWatcher_MissingTokenPath_Error verifies RunWatcher returns error on
// empty TokenPath.
func TestRunWatcher_MissingTokenPath_Error(t *testing.T) {
	err := RunWatcher(context.Background(), WatcherConfig{
		QueueDir:  "/some/dir",
		TokenPath: "",
	})
	if err == nil {
		t.Error("RunWatcher with empty TokenPath = nil, want error")
	}
}

// TestWriteJob_EmptyID_Error verifies that WriteJob returns an error when
// job.ID is empty (fail closed: an empty ID would write ".job" which
// PendingJobs silently skips).
func TestWriteJob_EmptyID_Error(t *testing.T) {
	dir := t.TempDir()
	err := WriteJob(dir, Job{
		ID:     "",
		ChatID: "-100",
		Text:   "should fail",
	})
	if err == nil {
		t.Error("WriteJob with empty ID = nil, want error")
	}
}

// TestPendingJobs_NonExistentDir_ReturnsNil verifies that PendingJobs on a
// non-existent directory returns (nil, nil) instead of an error.
func TestPendingJobs_NonExistentDir_ReturnsNil(t *testing.T) {
	pending, err := PendingJobs("/tmp/does-not-exist-qtest-" + t.Name())
	if err != nil {
		t.Errorf("PendingJobs on non-existent dir = error %v, want nil", err)
	}
	if len(pending) != 0 {
		t.Errorf("PendingJobs on non-existent dir = %v, want empty", pending)
	}
}

// TestPendingJobs_JobWithStatus_Excluded verifies that a job whose status file
// exists is NOT returned by PendingJobs.
func TestPendingJobs_JobWithStatus_Excluded(t *testing.T) {
	dir := t.TempDir()
	job := Job{
		ID:        "done-job-tg",
		ChatID:    "-100",
		Text:      "done",
		CreatedAt: time.Now().UTC(),
	}
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
	if err := WriteStatus(dir, JobStatus{
		ID:          job.ID,
		OK:          true,
		CompletedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}

	pending, err := PendingJobs(dir)
	if err != nil {
		t.Fatalf("PendingJobs: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("PendingJobs with status = %v, want empty (job already done)", pending)
	}
}

// TestPendingJobs_JobWithoutStatus_Included verifies that a job without a
// status file is returned by PendingJobs.
func TestPendingJobs_JobWithoutStatus_Included(t *testing.T) {
	dir := t.TempDir()
	job := Job{
		ID:        "pending-job-tg",
		ChatID:    "-100",
		Text:      "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	pending, err := PendingJobs(dir)
	if err != nil {
		t.Fatalf("PendingJobs: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("PendingJobs without status = %d, want 1", len(pending))
	}
}

// TestWriteJob_ReadJob_RoundTrip verifies the full WriteJob→ReadJob round-trip.
func TestWriteJob_ReadJob_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Job{
		ID:        "rt-001",
		ChatID:    "-100rt",
		Text:      "round-trip",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := WriteJob(dir, want); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
	got, err := ReadJob(filepath.Join(dir, want.ID+jobSuffix))
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}
	if got.ID != want.ID || got.ChatID != want.ChatID || got.Text != want.Text {
		t.Errorf("ReadJob = %+v, want %+v", got, want)
	}
}

// TestReadJob_MissingFile_Error verifies ReadJob returns an error for a
// non-existent file.
func TestReadJob_MissingFile_Error(t *testing.T) {
	_, err := ReadJob("/tmp/nonexistent-job-qtest.job")
	if err == nil {
		t.Error("ReadJob for missing file = nil, want error")
	}
}

// TestReadJob_BadJSON_Error verifies ReadJob returns an error for invalid JSON.
func TestReadJob_BadJSON_Error(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.job")
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadJob(path)
	if err == nil {
		t.Error("ReadJob for bad JSON = nil, want error")
	}
}

// TestWriteJob_MkdirAll_CreatesDir verifies WriteJob creates the directory.
func TestWriteJob_MkdirAll_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "outbox")
	job := Job{
		ID:        "mkdir-job",
		ChatID:    "-100",
		Text:      "mkdir test",
		CreatedAt: time.Now().UTC(),
	}
	if err := WriteJob(nested, job); err != nil {
		t.Fatalf("WriteJob with nested dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(nested, job.ID+jobSuffix)); err != nil {
		t.Errorf("job file not found after WriteJob with nested dir: %v", err)
	}
}

// TestWriteStatus_MkdirNotNeeded verifies WriteStatus writes into an existing dir.
func TestWriteStatus_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := JobStatus{
		ID:          "st-rt-001",
		OK:          true,
		CompletedAt: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
	}
	if err := WriteStatus(dir, want); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	got, err := ReadStatus(dir, want.ID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if got.ID != want.ID || got.OK != want.OK {
		t.Errorf("ReadStatus = %+v, want %+v", got, want)
	}
}

// TestWriteStatus_NonExistentDir_Error verifies WriteStatus returns an error
// when the directory does not exist (os.CreateTemp fails).
func TestWriteStatus_NonExistentDir_Error(t *testing.T) {
	err := WriteStatus("/tmp/no-such-dir-qtest-99/subdir", JobStatus{ID: "x", OK: true})
	if err == nil {
		t.Error("WriteStatus on non-existent dir = nil, want error")
	}
}

// TestWriteJob_ReadOnlyDir_Error verifies WriteJob returns an error when
// os.CreateTemp fails because the directory is read-only.
func TestWriteJob_ReadOnlyDir_Error(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root, read-only dir test not meaningful")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := WriteJob(dir, Job{
		ID:     "ro-job",
		ChatID: "-100",
		Text:   "should fail",
	})
	if err == nil {
		t.Error("WriteJob on read-only dir = nil, want error")
	}
}

// TestWriteStatus_ReadOnlyDir_Error verifies WriteStatus returns an error when
// os.CreateTemp fails because the directory is read-only.
func TestWriteStatus_ReadOnlyDir_Error(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root, read-only dir test not meaningful")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	err := WriteStatus(dir, JobStatus{ID: "ro-status", OK: true})
	if err == nil {
		t.Error("WriteStatus on read-only dir = nil, want error")
	}
}

// TestRunWatcher_ProcessQueueError_Continues verifies that RunWatcher logs
// a queue-scan error but keeps running (does not stop the loop).
// We test this by cancelling the context after the error log path fires.
func TestRunWatcher_ProcessQueueError_Continues(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root, permission test not meaningful")
	}
	// Create a queue dir then make it unreadable to trigger a scan error.
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "outbox")
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(queueDir, 0o111); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(queueDir, 0o755) })

	tokPath := writeTokenFile(t, "tok-rw-err")
	cfg := WatcherConfig{
		QueueDir:      queueDir,
		TokenPath:     tokPath,
		AllowedChatID: "-100",
		PollInterval:  50 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// RunWatcher must return nil (ctx cancelled), not an error from the scan.
	if err := RunWatcher(ctx, cfg); err != nil {
		t.Errorf("RunWatcher = %v, want nil (ctx cancel trumps scan error)", err)
	}
}

// TestProcessQueue_PendingJobsError_ReturnsError verifies processQueue returns
// an error when PendingJobs fails (non-NotExist error).
func TestProcessQueue_PendingJobsError_ReturnsError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root, permission test not meaningful")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o111); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	tokPath := writeTokenFile(t, "tok-pq-err")
	ob, err := NewOutbox(tokPath, "-100", alwaysConfirm)
	if err != nil {
		t.Fatalf("NewOutbox: %v", err)
	}
	if err := processQueue(context.Background(), dir, ob); err == nil {
		t.Error("processQueue on unreadable dir = nil, want error")
	}
}

// TestProcessQueue_CancelledWithPendingJob_ReturnsCtxErr verifies that
// processQueue returns ctx.Err() when the context is cancelled while iterating
// over pending jobs.
func TestProcessQueue_CancelledWithPendingJob_ReturnsCtxErr(t *testing.T) {
	dir := t.TempDir()
	job := Job{
		ID:        "cancel-job",
		ChatID:    "-100",
		Text:      "cancel test",
		CreatedAt: time.Now().UTC(),
	}
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}

	tokPath := writeTokenFile(t, "tok-cancel")
	ob, err := NewOutbox(tokPath, "-100", alwaysConfirm)
	if err != nil {
		t.Fatalf("NewOutbox: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before processQueue runs

	err = processQueue(ctx, dir, ob)
	if err == nil {
		t.Error("processQueue with cancelled ctx = nil, want error")
	}
}
