package async

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(10 * time.Minute)
	defer m.Close()

	if m.ttl != 10*time.Minute {
		t.Errorf("ttl = %v, want 10m", m.ttl)
	}
	if m.jobs == nil {
		t.Error("jobs map is nil")
	}
}

func TestStartAndGet(t *testing.T) {
	m := NewManager(1 * time.Hour)
	defer m.Close()

	done := make(chan struct{})
	job := m.Start("testing", func(j *Job) {
		ResultText(j, "hello world")
		close(done)
	})

	if job.ID == "" {
		t.Error("job ID is empty")
	}
	// Status is set before the goroutine starts, so it should be
	// "pending" at this point (the goroutine hasn't had time to run yet).

	// Wait for fn to complete before reading shared fields.
	<-done

	got, err := m.Get(job.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Result != "hello world" {
		t.Errorf("result = %q, want %q", got.Result, "hello world")
	}
	if got.Status != StatusCompleted {
		t.Errorf("status = %s, want completed", got.Status)
	}
}

func TestList(t *testing.T) {
	m := NewManager(1 * time.Hour)
	defer m.Close()

	ch := make(chan struct{}, 3)
	for i := 0; i < 3; i++ {
		m.Start("listing", func(j *Job) {
			ResultText(j, "ok")
			ch <- struct{}{}
		})
	}
	// Wait for all to finish.
	for i := 0; i < 3; i++ {
		<-ch
	}

	jobs := m.List()
	if len(jobs) != 3 {
		t.Errorf("List len = %d, want 3", len(jobs))
	}
}

func TestGet_NotFound(t *testing.T) {
	m := NewManager(1 * time.Hour)
	defer m.Close()

	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown ID, got nil")
	}
}

func TestResultJSON(t *testing.T) {
	job := &Job{ID: "test-1", Status: StatusPending}
	data := map[string]string{"key": "value"}

	ResultJSON(job, data)

	if job.Status != StatusCompleted {
		t.Errorf("status = %s, want completed", job.Status)
	}
	if job.Result != `{
  "key": "value"
}` {
		t.Errorf("result = %q, want JSON", job.Result)
	}
}

func TestResultJSON_MarshalError(t *testing.T) {
	job := &Job{ID: "test-2", Status: StatusPending}
	// Passing a channel causes json.Marshal to fail.
	ResultJSON(job, make(chan int))

	if job.Status != StatusFailed {
		t.Errorf("status = %s, want failed", job.Status)
	}
	if job.Error == "" {
		t.Error("expected error message, got empty")
	}
}

func TestResultText(t *testing.T) {
	job := &Job{ID: "test-3", Status: StatusPending}

	ResultText(job, "some text result")

	if job.Status != StatusCompleted {
		t.Errorf("status = %s, want completed", job.Status)
	}
	if job.Result != "some text result" {
		t.Errorf("result = %q, want %q", job.Result, "some text result")
	}
}

func TestFail(t *testing.T) {
	job := &Job{ID: "test-4", Status: StatusRunning}

	Fail(job, errors.New("something went wrong"))

	if job.Status != StatusFailed {
		t.Errorf("status = %s, want failed", job.Status)
	}
	if job.Error != "something went wrong" {
		t.Errorf("error = %q, want %q", job.Error, "something went wrong")
	}
}

func TestReap(t *testing.T) {
	// Use short TTL so jobs expire quickly, but long enough that the goroutine
	// can finish writing before the TTL has passed.
	m := NewManager(50 * time.Millisecond)
	defer m.Close()

	ch := make(chan struct{})
	m.Start("ephemeral", func(j *Job) {
		ResultText(j, "done")
		close(ch)
	})
	<-ch // Wait for fn(job) to return; goroutine finishes UpdatedAt right after.

	// Wait for TTL to expire.
	time.Sleep(60 * time.Millisecond)

	// Directly call reap — all goroutines are done, no data race.
	m.reap()

	if len(m.List()) != 0 {
		t.Errorf("expected 0 jobs after reaping, got %d", len(m.List()))
	}
}

func TestReap_KeepsRecent(t *testing.T) {
	m := NewManager(1 * time.Hour) // Long TTL, so nothing expires.
	defer m.Close()

	m.reap()
	// No jobs should be removed since there's nothing to reap.
	// This test just validates that reap doesn't panic with empty state.
}

func TestClose(t *testing.T) {
	m := NewManager(1 * time.Hour)
	m.Close()
	// Second Close should be safe (cancel is idempotent via context.CancelFunc docs).
	m.Close()
	// List/Get should still work after close.
	jobs := m.List()
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
	_, err := m.Get("any")
	if err == nil {
		t.Error("expected error for missing job, got nil")
	}
}

func TestConcurrentStart(t *testing.T) {
	m := NewManager(1 * time.Hour)
	defer m.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done := make(chan struct{})
			m.Start("concurrent", func(j *Job) {
				ResultText(j, "ok")
				close(done)
			})
			<-done
		}()
	}
	wg.Wait()

	jobs := m.List()
	if len(jobs) != 10 {
		t.Errorf("expected 10 jobs, got %d", len(jobs))
	}
}
