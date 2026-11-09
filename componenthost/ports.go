// Package componenthost defines the neutral capabilities used by installed
// component lifecycles. Composition and admission remain with the host owner.
package componenthost

import (
	"context"
	"net/http"
	"time"

	"github.com/MalenkiySolovey/solovey-host/auditrecord"
	"github.com/MalenkiySolovey/solovey-host/hostref"
	"github.com/MalenkiySolovey/solovey-host/observation"
	"github.com/robfig/cron/v3"
)

type Scheduler interface {
	AddJob(string, cron.Job) (cron.EntryID, error)
	Schedule(cron.Schedule, cron.Job) cron.EntryID
	RemoveJob(cron.EntryID)
	RemoveJobAndWait(context.Context, cron.EntryID) error
}

// OutboundHTTPClientProvider resolves the caller's explicitly selected outbound.
// The caller closes idle connections; request contexts retain cancellation.
type OutboundHTTPClientProvider interface {
	CoreHTTPClient(tag string, timeout time.Duration) (*http.Client, error)
}

type OutboundHTTPClientFunc func(string, time.Duration) (*http.Client, error)

func (f OutboundHTTPClientFunc) CoreHTTPClient(tag string, timeout time.Duration) (*http.Client, error) {
	return f(tag, timeout)
}

type AuditRecorder interface {
	Record(auditrecord.Event) error
}

// Ports contains only the existing capabilities used by component Start/Stop
// and owner-bound data drop. HTTP authorization and storage adapters stay with
// composition. Undeclared or unavailable capabilities must not be populated.
type Ports struct {
	Host               hostref.Ref
	Scheduler          Scheduler
	OutboundHTTP       OutboundHTTPClientProvider
	AuditRecorder      AuditRecorder
	ObservationSampler observation.Sampler
	DropOwnedSettings  func(context.Context) error
}

type Lifecycle interface {
	Start(context.Context, Ports) error
	Stop(context.Context) error
}

type DataDropper interface {
	DropData(context.Context, Ports) error
}
