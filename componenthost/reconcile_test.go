package componenthost

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

type lifecycleFixture struct {
	id                string
	calls             *[]string
	starts, stops     int
	startErr, stopErr error
	panicStart        bool
}

func (f *lifecycleFixture) Start(ctx context.Context, _ Ports) error {
	f.starts++
	*f.calls = append(*f.calls, "start:"+f.id)
	if f.panicStart {
		panic("fixture panic")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.startErr
}
func (f *lifecycleFixture) Stop(context.Context) error {
	f.stops++
	*f.calls = append(*f.calls, "stop:"+f.id)
	return f.stopErr
}
func lifecycleEntry(id string, calls *[]string) (Entry, *lifecycleFixture) {
	f := &lifecycleFixture{id: id, calls: calls}
	return Entry{ID: id, Lifecycle: f, Ports: Ports{Host: hostref.Local()}}, f
}

type lifecycleRecords struct{ start, stop map[string]bool }

func (r lifecycleRecords) RecordStart(id string, ok bool) { r.start[id] = ok }
func (r lifecycleRecords) RecordStop(id string, ok bool)  { r.stop[id] = ok }

func TestLifecycleReusesExistingInstanceAndPreservesStopOrder(t *testing.T) {
	ctx := context.Background()
	calls := []string{}
	a, first := lifecycleEntry("first", &calls)
	b, second := lifecycleEntry("second", &calls)
	running, err := Reconcile(ctx, nil, []Entry{a, b}, nil)
	if err != nil || len(running) != 2 {
		t.Fatal(err)
	}
	replacement, _ := lifecycleEntry("first", &calls)
	running, err = Reconcile(ctx, running, []Entry{replacement, b}, nil)
	if err != nil || running[0].Lifecycle != first || first.starts != 1 || second.starts != 1 {
		t.Fatal("idempotent reconciliation replaced runtime", err)
	}
	running, err = Stop(ctx, running, nil)
	if err != nil || len(running) != 0 || !reflect.DeepEqual(calls, []string{"start:first", "start:second", "stop:second", "stop:first"}) {
		t.Fatal(calls, err)
	}
	_, err = Stop(ctx, running, nil)
	if err != nil || first.stops != 1 || second.stops != 1 {
		t.Fatal("repeated stop called released instances")
	}
}

func TestLifecycleFailedShutdownRetainsInstanceUntilDrain(t *testing.T) {
	calls := []string{}
	entry, owner := lifecycleEntry("feature", &calls)
	record := lifecycleRecords{start: map[string]bool{}, stop: map[string]bool{}}
	running, err := Reconcile(context.Background(), nil, []Entry{entry}, record)
	if err != nil {
		t.Fatal(err)
	}
	owner.stopErr = context.DeadlineExceeded
	running, err = Reconcile(context.Background(), running, nil, record)
	if !errors.Is(err, context.DeadlineExceeded) || len(running) != 1 || record.stop[entry.ID] {
		t.Fatal("incomplete shutdown lost authority", err)
	}
	owner.stopErr = nil
	running, err = Stop(context.Background(), running, record)
	if err != nil || len(running) != 0 || !record.stop[entry.ID] || owner.stops != 2 {
		t.Fatal("failed shutdown not retried", err)
	}
}

func TestLifecycleAbsentDisabledAndMissingCapabilitiesDoNotStart(t *testing.T) {
	calls := []string{}
	entry, owner := lifecycleEntry("feature", &calls)
	for _, state := range []string{"ABSENT", "INSTALLED_DISABLED"} {
		t.Run(state, func(t *testing.T) {
			running, err := Reconcile(context.Background(), nil, nil, nil)
			if err != nil || len(running) != 0 || owner.starts != 0 {
				t.Fatal("inactive component activity", err)
			}
		})
	}
	owner.startErr = errors.New("required scheduler capability unavailable")
	record := lifecycleRecords{start: map[string]bool{}, stop: map[string]bool{}}
	running, err := Reconcile(context.Background(), nil, []Entry{entry}, record)
	if err == nil || len(running) != 0 || record.start[entry.ID] {
		t.Fatal("missing capability became started")
	}
	owner.startErr = nil
	owner.panicStart = true
	running, err = Reconcile(context.Background(), nil, []Entry{entry}, record)
	if err == nil || len(running) != 0 || record.start[entry.ID] {
		t.Fatal("panic became started")
	}
}

func TestLifecycleRejectsDuplicateOrForeignBindingsBeforeCalls(t *testing.T) {
	for _, kind := range []string{"duplicate", "host", "owner", "missing"} {
		t.Run(kind, func(t *testing.T) {
			calls := []string{}
			entry, _ := lifecycleEntry("feature", &calls)
			active := []Entry{entry}
			switch kind {
			case "duplicate":
				active = append(active, entry)
			case "host":
				active[0].Ports.Host.ID = "foreign"
			case "owner":
				active[0].ID = "../foreign"
			case "missing":
				active[0].Lifecycle = nil
			}
			if _, err := Reconcile(context.Background(), nil, active, nil); err == nil || len(calls) != 0 {
				t.Fatal("invalid binding dispatched")
			}
		})
	}
}

func TestLifecycleCancelledStartDoesNotPublishCallOrCreateWorker(t *testing.T) {
	calls := []string{}
	entry, _ := lifecycleEntry("feature", &calls)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	record := lifecycleRecords{start: map[string]bool{}, stop: map[string]bool{}}
	running, err := Reconcile(ctx, nil, []Entry{entry}, record)
	if !errors.Is(err, context.Canceled) || len(running) != 0 || len(calls) != 0 || len(record.start) != 0 {
		t.Fatal("cancelled startup fabricated lifecycle result", err)
	}
}
