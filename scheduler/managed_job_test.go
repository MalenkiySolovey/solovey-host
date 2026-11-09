package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type jobFunc func()

func (f jobFunc) Run() { f() }

type contextJobFixture struct {
	started chan struct{}
	finish  <-chan struct{}
}

func (j contextJobFixture) Run() { panic("context-aware job used the context-free entry") }

func (j contextJobFixture) RunContext(ctx context.Context) {
	close(j.started)
	if j.finish == nil {
		<-ctx.Done()
	} else {
		<-j.finish
	}
}

func TestManagedJobCancellationDrainsOperationLease(t *testing.T) {
	var acquired, released atomic.Int32
	job := contextJobFixture{started: make(chan struct{})}
	managed := NewManagedJob(t.Context(), job, func(ctx context.Context) (context.Context, func(), error) {
		acquired.Add(1)
		return ctx, func() { released.Add(1) }, nil
	})
	go managed.Run()
	select {
	case <-job.started:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not start")
	}
	managed.CancelAndPrevent()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if err := managed.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	managed.Run()
	if acquired.Load() != 1 || released.Load() != 1 {
		t.Fatal("cancelled job restarted or lost its operation lease")
	}
}

func TestManagedJobDeadlineRetainsInFlightOperation(t *testing.T) {
	finish := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(finish) }) }
	t.Cleanup(release)
	var released atomic.Bool
	job := contextJobFixture{started: make(chan struct{}), finish: finish}
	managed := NewManagedJob(t.Context(), job, func(ctx context.Context) (context.Context, func(), error) {
		return ctx, func() { released.Store(true) }, nil
	})
	go managed.Run()
	select {
	case <-job.started:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not start")
	}
	managed.CancelAndPrevent()
	short, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	if err := managed.Wait(short); !errors.Is(err, context.DeadlineExceeded) || released.Load() {
		t.Fatal("deadline reported the in-flight operation drained")
	}
	release()
	ctx, done := context.WithTimeout(t.Context(), 3*time.Second)
	defer done()
	if err := managed.Wait(ctx); err != nil || !released.Load() {
		t.Fatal("retry did not drain the retained operation")
	}
}

func TestManagedJobRejectsMissingOrInvalidAdmission(t *testing.T) {
	var zero ManagedJob
	zero.Run()
	zero.CancelAndPrevent()
	if err := zero.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, lease := range []OperationLease{
		nil,
		func(context.Context) (context.Context, func(), error) { return nil, nil, errors.New("unavailable") },
		func(context.Context) (context.Context, func(), error) { return nil, func() {}, nil },
		func(ctx context.Context) (context.Context, func(), error) { return ctx, nil, nil },
	} {
		called := false
		NewManagedJob(t.Context(), jobFunc(func() { called = true }), lease).Run()
		if called {
			t.Fatal("missing owner admission ran a job")
		}
	}
	var called bool
	managed := NewManagedJob(t.Context(), jobFunc(func() { called = true }), func(context.Context) (context.Context, func(), error) {
		t.Fatal("stopped job acquired an operation lease")
		return nil, nil, nil
	})
	managed.CancelAndPrevent()
	managed.Run()
	if err := managed.Wait(t.Context()); err != nil || called {
		t.Fatal("inactive job performed work")
	}
}

func TestManagedJobReleasesAdmissionWhenOwnerPanics(t *testing.T) {
	released := false
	managed := NewManagedJob(t.Context(), jobFunc(func() { panic("owner failure") }), func(ctx context.Context) (context.Context, func(), error) {
		return ctx, func() { released = true }, nil
	})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("owner panic was silently swallowed")
			}
		}()
		managed.Run()
	}()
	managed.CancelAndPrevent()
	if err := managed.Wait(t.Context()); err != nil || !released {
		t.Fatal("owner panic retained its admission")
	}
}

func TestManagedJobConcurrentCancellationAndWait(t *testing.T) {
	job := contextJobFixture{started: make(chan struct{})}
	managed := NewManagedJob(t.Context(), job, func(ctx context.Context) (context.Context, func(), error) {
		return ctx, func() {}, nil
	})
	go managed.Run()
	select {
	case <-job.started:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not start")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	var waits sync.WaitGroup
	for index := 0; index < 32; index++ {
		waits.Add(1)
		go func() {
			defer waits.Done()
			managed.CancelAndPrevent()
			if err := managed.Wait(ctx); err != nil {
				t.Error(err)
			}
		}()
	}
	waits.Wait()
}
