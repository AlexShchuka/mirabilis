package notify

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const (
	OutboxSubdir = ".mirabilis/outbox"

	jobSuffix    = ".job"
	statusSuffix = ".status"
)

type Job struct {
	ID        string    `json:"id"`
	ChatID    string    `json:"chat_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type JobStatus struct {
	ID          string    `json:"id"`
	OK          bool      `json:"ok"`
	Error       string    `json:"error,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
}

func OutboxDir(repoRoot string) string {
	return filepath.Join(repoRoot, OutboxSubdir)
}

func WriteJob(dir string, job Job) error {
	if job.ID == "" {
		return errors.New("notify queue: WriteJob: job.ID must not be empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("notify queue: mkdir %q: %w", dir, err)
	}
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("notify queue: marshal job: %w", err)
	}
	return writeAtomic(dir, job.ID+jobSuffix, ".job-*.tmp", data)
}

func ReadJob(path string) (Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Job{}, fmt.Errorf("notify queue: read job %q: %w", path, err)
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return Job{}, fmt.Errorf("notify queue: parse job %q: %w", path, err)
	}
	return job, nil
}

func WriteStatus(dir string, status JobStatus) error {
	if status.ID == "" {
		return errors.New("notify queue: refusing to write status with empty job ID")
	}
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("notify queue: marshal status: %w", err)
	}
	return writeAtomic(dir, status.ID+statusSuffix, ".status-*.tmp", data)
}

func ReadStatus(dir, jobID string) (JobStatus, error) {
	data, err := os.ReadFile(filepath.Join(dir, jobID+statusSuffix))
	if err != nil {
		return JobStatus{}, err
	}
	var s JobStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return JobStatus{}, fmt.Errorf("notify queue: parse status: %w", err)
	}
	return s, nil
}

func PendingJobs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("notify queue: read dir %q: %w", dir, err)
	}
	statusSet := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == statusSuffix {
			statusSet[e.Name()[:len(e.Name())-len(statusSuffix)]] = true
		}
	}
	var pending []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == jobSuffix {
			id := e.Name()[:len(e.Name())-len(jobSuffix)]
			if !statusSet[id] {
				pending = append(pending, filepath.Join(dir, e.Name()))
			}
		}
	}
	return pending, nil
}

func writeAtomic(dir, name, tmpPattern string, data []byte) error {
	dest := filepath.Join(dir, name)
	tmp, err := os.CreateTemp(dir, tmpPattern)
	if err != nil {
		return fmt.Errorf("notify queue: create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("notify queue: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("notify queue: close temp: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("notify queue: rename to %q: %w", dest, err)
	}
	return nil
}
