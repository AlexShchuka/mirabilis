package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	legacyJobJSON        = `{"id":"job-1","chat_id":"-1001","text":"hi","created_at":"2026-01-02T03:04:05Z"}`
	legacyStatusOKJSON   = `{"id":"job-1","ok":true,"completed_at":"2026-01-02T03:04:06Z"}`
	legacyStatusFailJSON = `{"id":"job-2","ok":false,"error":"boom","completed_at":"2026-01-02T03:04:06Z"}`
)

func TestJobJSONGolden(t *testing.T) {
	job := Job{
		ID:        "job-1",
		ChatID:    "-1001",
		Text:      "hi",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != legacyJobJSON {
		t.Errorf("marshal = %s, want %s", data, legacyJobJSON)
	}
	var got Job
	if err := json.Unmarshal([]byte(legacyJobJSON), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != job.ID || got.ChatID != job.ChatID || got.Text != job.Text || !got.CreatedAt.Equal(job.CreatedAt) {
		t.Errorf("unmarshal = %+v, want %+v", got, job)
	}
}

func TestJobStatusJSONGolden(t *testing.T) {
	okStatus := JobStatus{
		ID:          "job-1",
		OK:          true,
		CompletedAt: time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC),
	}
	data, err := json.Marshal(okStatus)
	if err != nil {
		t.Fatalf("marshal ok: %v", err)
	}
	if string(data) != legacyStatusOKJSON {
		t.Errorf("marshal ok = %s, want %s", data, legacyStatusOKJSON)
	}

	failStatus := JobStatus{
		ID:          "job-2",
		OK:          false,
		Error:       "boom",
		CompletedAt: time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC),
	}
	data, err = json.Marshal(failStatus)
	if err != nil {
		t.Fatalf("marshal fail: %v", err)
	}
	if string(data) != legacyStatusFailJSON {
		t.Errorf("marshal fail = %s, want %s", data, legacyStatusFailJSON)
	}

	var got JobStatus
	if err := json.Unmarshal([]byte(legacyStatusFailJSON), &got); err != nil {
		t.Fatalf("unmarshal fail: %v", err)
	}
	if got.ID != failStatus.ID || got.OK || got.Error != "boom" || !got.CompletedAt.Equal(failStatus.CompletedAt) {
		t.Errorf("unmarshal fail = %+v, want %+v", got, failStatus)
	}
}

func TestQueueRoundTrip(t *testing.T) {
	dir := t.TempDir()
	job := Job{
		ID:        "rt-1",
		ChatID:    "-100rt",
		Text:      "round-trip",
		CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
	gotJob, err := ReadJob(filepath.Join(dir, job.ID+jobSuffix))
	if err != nil {
		t.Fatalf("ReadJob: %v", err)
	}
	if gotJob.ID != job.ID || gotJob.ChatID != job.ChatID || gotJob.Text != job.Text || !gotJob.CreatedAt.Equal(job.CreatedAt) {
		t.Errorf("ReadJob = %+v, want %+v", gotJob, job)
	}

	status := JobStatus{
		ID:          job.ID,
		OK:          true,
		CompletedAt: time.Date(2026, 1, 2, 3, 4, 6, 0, time.UTC),
	}
	if err := WriteStatus(dir, status); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	gotStatus, err := ReadStatus(dir, job.ID)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if gotStatus.ID != status.ID || !gotStatus.OK || !gotStatus.CompletedAt.Equal(status.CompletedAt) {
		t.Errorf("ReadStatus = %+v, want %+v", gotStatus, status)
	}
}

func TestQueueAtomicWritesLeaveNoTemp(t *testing.T) {
	dir := t.TempDir()
	job := Job{ID: "atomic-1", ChatID: "-100", Text: "x", CreatedAt: time.Now().UTC()}
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
	if err := WriteStatus(dir, JobStatus{ID: job.ID, OK: true, CompletedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("dir has %d entries, want 2 (job + status)", len(entries))
	}
}

func TestWriteJobCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "outbox")
	job := Job{ID: "mkdir-1", ChatID: "-100", Text: "x", CreatedAt: time.Now().UTC()}
	if err := WriteJob(dir, job); err != nil {
		t.Fatalf("WriteJob: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, job.ID+jobSuffix)); err != nil {
		t.Errorf("job file missing: %v", err)
	}
}

func TestWriteJobEmptyIDRejected(t *testing.T) {
	if err := WriteJob(t.TempDir(), Job{ChatID: "-100", Text: "x"}); err == nil {
		t.Error("WriteJob with empty ID = nil, want error")
	}
}

func TestWriteStatusEmptyIDRejected(t *testing.T) {
	if err := WriteStatus(t.TempDir(), JobStatus{OK: true}); err == nil {
		t.Error("WriteStatus with empty ID = nil, want error")
	}
}

func TestPendingJobsFiltering(t *testing.T) {
	dir := t.TempDir()
	pendingJob := Job{ID: "pending-1", ChatID: "-100", Text: "p", CreatedAt: time.Now().UTC()}
	doneJob := Job{ID: "done-1", ChatID: "-100", Text: "d", CreatedAt: time.Now().UTC()}
	for _, j := range []Job{pendingJob, doneJob} {
		if err := WriteJob(dir, j); err != nil {
			t.Fatalf("WriteJob %s: %v", j.ID, err)
		}
	}
	if err := WriteStatus(dir, JobStatus{ID: doneJob.ID, OK: true, CompletedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("WriteStatus: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pending, err := PendingJobs(dir)
	if err != nil {
		t.Fatalf("PendingJobs: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("PendingJobs = %v, want exactly the pending job", pending)
	}
	if want := filepath.Join(dir, pendingJob.ID+jobSuffix); pending[0] != want {
		t.Errorf("pending = %q, want %q", pending[0], want)
	}
}

func TestPendingJobsMissingDir(t *testing.T) {
	pending, err := PendingJobs(filepath.Join(t.TempDir(), "no-such-dir"))
	if err != nil {
		t.Errorf("PendingJobs on missing dir = %v, want nil", err)
	}
	if len(pending) != 0 {
		t.Errorf("PendingJobs on missing dir = %v, want empty", pending)
	}
}

func TestOutboxDir(t *testing.T) {
	got := OutboxDir("/home/user/project")
	want := filepath.Join("/home/user/project", ".mirabilis", "outbox")
	if got != want {
		t.Errorf("OutboxDir = %q, want %q", got, want)
	}
}

func TestReadJobErrors(t *testing.T) {
	if _, err := ReadJob(filepath.Join(t.TempDir(), "missing.job")); err == nil {
		t.Error("ReadJob missing file = nil, want error")
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.job")
	if err := os.WriteFile(bad, []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadJob(bad); err == nil {
		t.Error("ReadJob bad JSON = nil, want error")
	}
}

func TestReadStatusErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadStatus(dir, "missing"); err == nil {
		t.Error("ReadStatus missing file = nil, want error")
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.status"), []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadStatus(dir, "bad"); err == nil {
		t.Error("ReadStatus bad JSON = nil, want error")
	}
}

func TestPruneDelivered(t *testing.T) {
	dir := t.TempDir()
	if err := WriteJob(dir, Job{ID: "old", ChatID: "-1", Text: "x", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := WriteStatus(dir, JobStatus{ID: "old", OK: true, CompletedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	stale := time.Now().Add(-time.Hour)
	for _, name := range []string{"old.job", "old.status"} {
		if err := os.Chtimes(filepath.Join(dir, name), stale, stale); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteJob(dir, Job{ID: "fresh", ChatID: "-1", Text: "y", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := WriteStatus(dir, JobStatus{ID: "fresh", OK: true, CompletedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	pendingOnly := "pending"
	if err := WriteJob(dir, Job{ID: pendingOnly, ChatID: "-1", Text: "z", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	if err := PruneDelivered(dir, 10*time.Minute); err != nil {
		t.Fatalf("PruneDelivered: %v", err)
	}

	for _, name := range []string{"old.job", "old.status"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s still present after prune, want removed", name)
		}
	}
	for _, name := range []string{"fresh.job", "fresh.status", "pending.job"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s missing after prune, want kept: %v", name, err)
		}
	}
}

func TestPruneDeliveredMissingDir(t *testing.T) {
	if err := PruneDelivered(filepath.Join(t.TempDir(), "nope"), time.Minute); err != nil {
		t.Errorf("PruneDelivered(missing) = %v, want nil", err)
	}
}
