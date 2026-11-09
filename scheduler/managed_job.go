// Package scheduler provides cancellation and drain behavior for scheduled jobs.
// It does not create a scheduler, register jobs or start background work.
package scheduler

import (
	"context"
	"sync"

	"github.com/robfig/cron/v3"
)

// OperationLease holds the existing owner's admission for the entire operation.
// Success requires a non-nil context and release function. Storage generations
// and maintenance fences remain implemented by the injected owner.
type OperationLease func(context.Context) (context.Context, func(), error)

type contextJob interface {
	RunContext(context.Context)
}

type ManagedJob struct {
	job    cron.Job
	lease  OperationLease
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	stopped bool
	active  sync.WaitGroup
}

func NewManagedJob(parent context.Context, job cron.Job, lease OperationLease) *ManagedJob {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &ManagedJob{job: job, lease: lease, ctx: ctx, cancel: cancel}
}

func (j *ManagedJob) Run() {
	if j == nil || j.job == nil || j.lease == nil {
		return
	}
	j.mu.Lock()
	if j.stopped || j.ctx.Err() != nil {
		j.mu.Unlock()
		return
	}
	j.active.Add(1)
	j.mu.Unlock()
	defer j.active.Done()
	ctx, release, err := j.lease(j.ctx)
	if err != nil || ctx == nil || release == nil {
		if release != nil {
			release()
		}
		return
	}
	defer release()
	if job, ok := j.job.(contextJob); ok {
		job.RunContext(ctx)
		return
	}
	j.job.Run()
}

func (j *ManagedJob) CancelAndPrevent() {
	if j == nil {
		return
	}
	j.mu.Lock()
	j.stopped = true
	if j.cancel != nil {
		j.cancel()
	}
	j.mu.Unlock()
}

// Wait drains admitted runs after CancelAndPrevent. A deadline leaves the job
// available for a later wait; it never reports a still-running operation done.
func (j *ManagedJob) Wait(ctx context.Context) error {
	if j == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	done := make(chan struct{})
	go func() {
		j.active.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		select {
		case <-done:
			return nil
		default:
			return ctx.Err()
		}
	}
}
