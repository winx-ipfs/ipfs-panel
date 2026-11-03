package pins

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"
)

const (
	// MaxRunning is how many pin fetches may run at once.
	MaxRunning = 4
	// KeepFinished is how many finished jobs stay in memory.
	KeepFinished = 40
	// DefaultPinTimeout is the deadline for one pin/add.
	DefaultPinTimeout = 30 * time.Minute
)

// Job is the public view of one pin fetch.
type Job struct {
	ID       string
	CID      string
	Name     string
	Type     string
	State    string
	Progress int
	Error    string
	Created  time.Time
}

type entry struct {
	Job
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// Book tracks pin jobs in memory.
type Book struct {
	mu      sync.Mutex
	jobs    map[string]*entry
	running int
	max     int
	keep    int
}

// NewBook returns an empty book.
func NewBook(maxRunning, keepFinished int) *Book {
	if maxRunning <= 0 {
		maxRunning = MaxRunning
	}
	if keepFinished <= 0 {
		keepFinished = KeepFinished
	}
	return &Book{
		jobs: map[string]*entry{},
		max:  maxRunning,
		keep: keepFinished,
	}
}

// Start runs fn without waiting for it. The returned job is already running.
func (b *Book) Start(target, name string, recursive bool, timeout time.Duration, run func(context.Context, func(int)) (string, error)) (Job, error) {
	if timeout <= 0 {
		timeout = DefaultPinTimeout
	}
	id, err := newID()
	if err != nil {
		return Job{}, &OpError{Code: "unavailable"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	b.mu.Lock()
	if b.running >= b.max {
		b.mu.Unlock()
		cancel()
		return Job{}, &OpError{Code: "too_many"}
	}
	kind := "recursive"
	if !recursive {
		kind = "direct"
	}
	job := &entry{
		Job: Job{
			ID:      id,
			CID:     RootCID(target),
			Name:    name,
			Type:    kind,
			State:   "running",
			Created: time.Now().UTC(),
		},
		cancel: cancel,
		done:   make(chan struct{}),
	}
	b.jobs[id] = job
	b.running++
	snap := job.Job
	b.mu.Unlock()
	go func() {
		cid, runErr := run(ctx, func(n int) { b.progress(id, n) })
		b.finish(id, cid, ctx, runErr)
	}()
	return snap, nil
}

// Get returns one job.
func (b *Book) Get(id string) (Job, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	job, ok := b.jobs[id]
	if !ok {
		return Job{}, &OpError{Code: "not_found"}
	}
	return job.Job, nil
}

// List returns jobs, newest first.
func (b *Book) List() []Job {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Job, 0, len(b.jobs))
	for _, job := range b.jobs {
		out = append(out, job.Job)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Created.Equal(out[j].Created) {
			return out[i].ID > out[j].ID
		}
		return out[i].Created.After(out[j].Created)
	})
	return out
}

// Cancel asks a running job to stop. A finished job is returned unchanged.
func (b *Book) Cancel(id string) (Job, error) {
	b.mu.Lock()
	job, ok := b.jobs[id]
	if !ok {
		b.mu.Unlock()
		return Job{}, &OpError{Code: "not_found"}
	}
	if job.State != "running" {
		snap := job.Job
		b.mu.Unlock()
		return snap, nil
	}
	job.State = "cancelled"
	job.Error = "cancelled"
	if b.running > 0 {
		b.running--
	}
	cancel := job.cancel
	done := job.done
	b.pruneLocked()
	b.mu.Unlock()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	return b.Get(id)
}

func (b *Book) progress(id string, n int) {
	if n <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	job, ok := b.jobs[id]
	if !ok || job.State != "running" || n <= job.Progress {
		return
	}
	job.Progress = n
}

func (b *Book) finish(id, cid string, ctx context.Context, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	job, ok := b.jobs[id]
	if !ok {
		return
	}
	defer job.once.Do(func() { close(job.done) })
	if job.State != "running" {
		return
	}
	state, code := classifyJob(ctx, err)
	job.State = state
	job.Error = code
	if cid != "" {
		job.CID = cid
	}
	if b.running > 0 {
		b.running--
	}
	b.pruneLocked()
}

func classifyJob(ctx context.Context, err error) (string, string) {
	if err == nil {
		return "succeeded", ""
	}
	if errors.Is(ctx.Err(), context.Canceled) || CodeOf(err) == "cancelled" {
		return "cancelled", "cancelled"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || CodeOf(err) == "timeout" {
		return "failed", "timeout"
	}
	switch CodeOf(err) {
	case "not_found", "node_not_running", "unavailable", "kubo_error", "too_large", "indirect":
		return "failed", CodeOf(err)
	case "":
		return "failed", "kubo_error"
	default:
		return "failed", "kubo_error"
	}
}

func (b *Book) pruneLocked() {
	type item struct {
		id string
		at time.Time
	}
	finished := make([]item, 0)
	for id, job := range b.jobs {
		if job.State != "running" {
			finished = append(finished, item{id: id, at: job.Created})
		}
	}
	if len(finished) <= b.keep {
		return
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].at.Before(finished[j].at) })
	extra := len(finished) - b.keep
	for i := 0; i < extra; i++ {
		delete(b.jobs, finished[i].id)
	}
}

func newID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
