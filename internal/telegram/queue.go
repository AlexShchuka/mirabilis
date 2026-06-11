// Package telegram/queue defines the on-disk schema for the host↔container
// Telegram outbox queue.
//
// Layout (under <repo>/.mirabilis/outbox/):
//
//	<id>.job     – job to process; written atomically by tgsend inside the container
//	<id>.status  – outcome; written by the host-side watcher after delivery
//
// A job file is a JSON object; the status file is a JSON object.
// No token is ever stored in either file.
package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// OutboxSubdir is the subdir under the repo root where job files are queued.
	OutboxSubdir = ".mirabilis/outbox"

	jobSuffix    = ".job"
	statusSuffix = ".status"
)

// Job is the on-disk representation of a pending send.
// No token, no credentials — only data the container is allowed to produce.
type Job struct {
	ID        string    `json:"id"`
	ChatID    string    `json:"chat_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// JobStatus records the outcome written by the host-side watcher.
type JobStatus struct {
	ID          string    `json:"id"`
	OK          bool      `json:"ok"`
	Error       string    `json:"error,omitempty"`
	CompletedAt time.Time `json:"completed_at"`
}

// OutboxDir returns the absolute path to the outbox directory for the given
// repo root.
func OutboxDir(repoRoot string) string {
	return filepath.Join(repoRoot, OutboxSubdir)
}

// WriteJob atomically writes job to dir/<id>.job.
// The containing directory is created if it does not exist.
// Returns an error if job.ID is empty (fail closed: an empty ID would write
// ".job" which PendingJobs silently skips).
func WriteJob(dir string, job Job) error {
	if job.ID == "" {
		return fmt.Errorf("telegram queue: WriteJob: job.ID must not be empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("telegram queue: mkdir %q: %w", dir, err)
	}
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("telegram queue: marshal job: %w", err)
	}
	dest := filepath.Join(dir, job.ID+jobSuffix)
	tmp, err := os.CreateTemp(dir, ".job-*.tmp")
	if err != nil {
		return fmt.Errorf("telegram queue: create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("telegram queue: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("telegram queue: close temp: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("telegram queue: rename to %q: %w", dest, err)
	}
	return nil
}

// ReadJob reads and parses a job file.
func ReadJob(path string) (Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Job{}, fmt.Errorf("telegram queue: read job %q: %w", path, err)
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return Job{}, fmt.Errorf("telegram queue: parse job %q: %w", path, err)
	}
	return job, nil
}

// WriteStatus atomically writes a status file for the given job ID.
func WriteStatus(dir string, status JobStatus) error {
	if status.ID == "" {
		return fmt.Errorf("telegram queue: refusing to write status with empty job ID")
	}
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("telegram queue: marshal status: %w", err)
	}
	dest := filepath.Join(dir, status.ID+statusSuffix)
	tmp, err := os.CreateTemp(dir, ".status-*.tmp")
	if err != nil {
		return fmt.Errorf("telegram queue: create temp status: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("telegram queue: write temp status: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("telegram queue: close temp status: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("telegram queue: rename status to %q: %w", dest, err)
	}
	return nil
}

// ReadStatus reads a status file for the given job ID in dir.
func ReadStatus(dir, jobID string) (JobStatus, error) {
	path := filepath.Join(dir, jobID+statusSuffix)
	data, err := os.ReadFile(path)
	if err != nil {
		return JobStatus{}, err
	}
	var s JobStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return JobStatus{}, fmt.Errorf("telegram queue: parse status: %w", err)
	}
	return s, nil
}

// PendingJobs returns the paths to all unprocessed job files in dir.
// A job is unprocessed if no matching <id>.status file exists.
func PendingJobs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("telegram queue: read dir %q: %w", dir, err)
	}
	statusSet := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == statusSuffix {
			id := e.Name()[:len(e.Name())-len(statusSuffix)]
			statusSet[id] = true
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
