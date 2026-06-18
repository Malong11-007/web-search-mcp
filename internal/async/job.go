package async

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status represents the state of an async job.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Job holds the state of an asynchronous operation.
type Job struct {
	ID        string    `json:"id"`
	Status    Status    `json:"status"`
	Progress  string    `json:"progress,omitempty"`
	Result    string    `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager tracks async jobs in memory.
type Manager struct {
	mu     sync.RWMutex
	jobs   map[string]*Job
	ttl    time.Duration
	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager creates a job manager that cleans up jobs older than ttl.
func NewManager(ttl time.Duration) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		jobs:   make(map[string]*Job),
		ttl:    ttl,
		ctx:    ctx,
		cancel: cancel,
	}
	go m.reapLoop()
	return m
}

// Start creates a new job and runs fn in a goroutine.
// fn should write its result and update the job.
func (m *Manager) Start(progress string, fn func(job *Job)) *Job {
	job := &Job{
		ID:        uuid.New().String()[:8],
		Status:    StatusPending,
		Progress:  progress,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	go func() {
		job.Status = StatusRunning
		job.UpdatedAt = time.Now()
		fn(job)
		job.UpdatedAt = time.Now()
	}()

	return job
}

// Get returns a job by ID, or an error if not found.
func (m *Manager) Get(id string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %q not found", id)
	}
	return job, nil
}

// List returns all jobs.
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// Close stops the reaper goroutine.
func (m *Manager) Close() {
	m.cancel()
}

func (m *Manager) reapLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.reap()
		}
	}
}

func (m *Manager) reap() {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-m.ttl)
	for id, job := range m.jobs {
		if job.UpdatedAt.Before(cutoff) {
			delete(m.jobs, id)
		}
	}
}

// ResultJSON marshals result data to JSON and stores it in the job.
func ResultJSON(job *Job, data any) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		job.Status = StatusFailed
		job.Error = fmt.Sprintf("marshal result: %v", err)
		return
	}
	job.Status = StatusCompleted
	job.Result = string(b)
}

// ResultText stores a text result in the job.
func ResultText(job *Job, text string) {
	job.Status = StatusCompleted
	job.Result = text
}

// Fail marks the job as failed.
func Fail(job *Job, err error) {
	job.Status = StatusFailed
	job.Error = err.Error()
}
