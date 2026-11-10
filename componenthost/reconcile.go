package componenthost

import (
	"context"
	"errors"
	"fmt"

	"github.com/MalenkiySolovey/solovey-host/componenthost/manifest"
)

// Entry binds one installed lifecycle to its admitted local ports. Composition
// supplies only installed/enabled entries after capability and role admission.
// Absent entries must not be constructed or registered by an importer.
type Entry struct {
	ID        string
	Lifecycle Lifecycle
	Ports     Ports
}

// LifecycleRecorder publishes actual calls; Start returning nil is not a Health
// readiness assertion. Concrete observation storage remains with its owner.
type LifecycleRecorder interface {
	RecordStart(string, bool)
	RecordStop(string, bool)
}

// Reconcile is the existing supervisor's bounded reverse-stop/forward-start
// algorithm. The caller holds its existing lifecycle/removal serializer and
// retains the returned instances, including failed Stops, for the next retry.
// This function creates no registry, worker, scheduler, state store or lock.
// Start implementations must undo partial startup on failure; Stop must drain
// owned activity before returning nil. Disable never invokes DropData.
func Reconcile(ctx context.Context, running, active []Entry, observations LifecycleRecorder) ([]Entry, error) {
	if ctx == nil {
		return running, errors.New("component lifecycle context unavailable")
	}
	if err := validateEntries(running); err != nil {
		return running, err
	}
	if err := validateEntries(active); err != nil {
		return running, err
	}
	activeByID := make(map[string]Entry, len(active))
	for _, entry := range active {
		activeByID[entry.ID] = entry
	}
	runningByID := make(map[string]Entry, len(running))
	for _, entry := range running {
		runningByID[entry.ID] = entry
	}
	var joined error
	stopFailed := make(map[string]struct{})
	for i := len(running) - 1; i >= 0; i-- {
		entry := running[i]
		if _, ok := activeByID[entry.ID]; ok {
			continue
		}
		err := callLifecycle(entry.ID, "stop", func() error { return entry.Lifecycle.Stop(ctx) })
		if observations != nil {
			observations.RecordStop(entry.ID, err == nil)
		}
		if err != nil {
			joined = errors.Join(joined, err)
			stopFailed[entry.ID] = struct{}{}
		}
	}
	next := make([]Entry, 0, len(active)+len(stopFailed))
	for _, entry := range active {
		if current, ok := runningByID[entry.ID]; ok {
			next = append(next, current)
			continue
		}
		if err := ctx.Err(); err != nil {
			joined = errors.Join(joined, err)
			continue
		}
		err := callLifecycle(entry.ID, "start", func() error { return entry.Lifecycle.Start(ctx, entry.Ports) })
		if observations != nil {
			observations.RecordStart(entry.ID, err == nil)
		}
		if err != nil {
			joined = errors.Join(joined, err)
			continue
		}
		next = append(next, entry)
	}
	for _, entry := range running {
		if _, failed := stopFailed[entry.ID]; failed {
			next = append(next, entry)
		}
	}
	return next, joined
}

// Stop drains in reverse start order and retains only failed Stop instances.
// A cancelled/incomplete drain is a failure and remains retryable.
func Stop(ctx context.Context, running []Entry, observations LifecycleRecorder) ([]Entry, error) {
	return Reconcile(ctx, running, nil, observations)
}

func validateEntries(entries []Entry) error {
	if len(entries) > 64 {
		return errors.New("component lifecycle bound exceeded")
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if manifest.ValidateID(entry.ID) != nil || seen[entry.ID] || entry.Lifecycle == nil || !entry.Ports.Host.IsLocal() {
			return errors.New("component lifecycle identity or local binding unavailable")
		}
		seen[entry.ID] = true
	}
	return nil
}

func callLifecycle(id, action string, call func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("component %q %s panic: %v", id, action, recovered)
		}
	}()
	if err := call(); err != nil {
		return fmt.Errorf("component %q %s failed: %w", id, action, err)
	}
	return nil
}
