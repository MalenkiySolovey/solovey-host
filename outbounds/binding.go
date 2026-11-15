// Package outbounds admits source bindings through the existing Host-local
// Outbounds owner. It supplies no database, Core writer, worker or authorization.
package outbounds

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/MalenkiySolovey/solovey-host/coreruntime"
	"github.com/MalenkiySolovey/solovey-host/hostref"
)

type BindingRef struct {
	Host       hostref.Ref
	SourceID   string
	EndpointID string
}

type ResourceRef struct {
	Host hostref.Ref
	ID   uint
	Tag  string
}

type Action string

const (
	Upsert      Action = "UPSERT"
	MarkMissing Action = "MARK_MISSING"
	Remove      Action = "REMOVE"
)

// Missing describes source health, not routing or permission to delete a resource.
type Missing struct {
	Reason, Source string
	Since          int64
}

// Candidate is one Outbound projection. Definition is bounded resource JSON,
// never a complete Core configuration. Revisions must come from owner reads.
type Candidate struct {
	Binding                  BindingRef
	Resource                 ResourceRef
	Provider                 coreruntime.Provider
	SourceRevision           string
	ExpectedResourceRevision string
	Action                   Action
	Definition               json.RawMessage
	Missing                  Missing
}

type Admission struct {
	Binding          BindingRef
	Resource         ResourceRef
	Provider         coreruntime.Provider
	SourceRevision   string
	ResourceRevision string
	Capability       coreruntime.Capability
	Owned            bool
}

type State string

const (
	Pending State = "PENDING"
	Applied State = "APPLIED"
	Failed  State = "FAILED"
	Unknown State = "UNKNOWN"
)

type Observation struct {
	Binding          BindingRef
	Resource         ResourceRef
	Provider         coreruntime.Provider
	SourceRevision   string
	ResourceRevision string
	Present          bool
	Missing          bool
	// CandidateDigest is set only after the owner read back this exact write.
	// Source snapshot acceptance alone must never populate it.
	CandidateDigest string
}

type Receipt struct {
	Binding          BindingRef
	Resource         ResourceRef
	State            State
	ResourceRevision string
	CandidateDigest  string
	ApplyAttempted   bool
}

type Finalization string

const (
	TransactionPending    Finalization = "PENDING"
	TransactionCommitted  Finalization = "COMMITTED"
	TransactionRolledBack Finalization = "ROLLED_BACK"
	TransactionUnknown    Finalization = "UNKNOWN"
)

// Lease is scoped to one resource and source revision. The trusted owner holds
// its existing transaction/operation fence until finalization, validates the
// resource projection, preserves local fields and executes its canonical writer.
type Lease interface {
	Admission() Admission
	WriteBinding(context.Context, Candidate) error
	ObserveBinding(context.Context) (Observation, error)
}

type Owner interface {
	WithBindingLease(context.Context, Candidate, func(context.Context, Lease) error) (Finalization, error)
}

type Preview struct {
	Admission       Admission
	CandidateDigest string
}

type Query struct {
	Binding  BindingRef
	Resource ResourceRef
}
type Reader interface {
	ObserveBinding(context.Context, Query) (Observation, error)
}

var (
	ErrScope        = errors.New("outbound binding scope rejected")
	ErrRevision     = errors.New("outbound binding revision changed")
	ErrProvider     = errors.New("outbound binding provider changed")
	ErrCapability   = errors.New("outbound binding capability unavailable")
	ErrConflict     = errors.New("outbound binding writer conflict")
	ErrLease        = errors.New("outbound binding lease unavailable")
	ErrDefinition   = errors.New("outbound binding definition invalid")
	ErrObservation  = errors.New("outbound binding write unverified")
	ErrFinalization = errors.New("outbound binding transaction unverified")
)

const MaxDefinitionBytes = 512 * 1024

func token(s string) bool { return s != "" && len(s) <= 512 && !strings.ContainsAny(s, "\x00\r\n") }
func validQuery(q Query) bool {
	return q.Binding.Host.IsLocal() && q.Resource.Host == q.Binding.Host && token(q.Binding.SourceID) && token(q.Binding.EndpointID) && token(q.Resource.Tag)
}
func validate(c Candidate) error {
	if !validQuery(Query{c.Binding, c.Resource}) || !token(c.SourceRevision) || !token(c.ExpectedResourceRevision) {
		return ErrScope
	}
	if (c.Provider.ID != "sing-box" && c.Provider.ID != "shtorm-extended") || !token(c.Provider.Revision) {
		return ErrProvider
	}
	switch c.Action {
	case Upsert:
		if len(c.Definition) == 0 || len(c.Definition) > MaxDefinitionBytes {
			return ErrDefinition
		}
		var shape map[string]json.RawMessage
		if json.Unmarshal(c.Definition, &shape) != nil || shape == nil {
			return ErrDefinition
		}
		var tag, kind string
		if json.Unmarshal(shape["tag"], &tag) != nil || tag != c.Resource.Tag || json.Unmarshal(shape["type"], &kind) != nil || !token(kind) {
			return ErrDefinition
		}
		for _, key := range []string{"inbounds", "route", "dns", "experimental"} {
			if _, ok := shape[key]; ok {
				return ErrDefinition
			}
		}
	case MarkMissing:
		if len(c.Definition) != 0 || c.Missing.Since < 0 || len(c.Missing.Reason) > 4096 || len(c.Missing.Source) > 4096 {
			return ErrDefinition
		}
	case Remove:
		if len(c.Definition) != 0 {
			return ErrDefinition
		}
	default:
		return ErrDefinition
	}
	return nil
}

// Digest binds a request without returning its potentially secret definition.
func Digest(c Candidate) string {
	raw, _ := json.Marshal(c)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func checkAdmission(c Candidate, a Admission) error {
	if a.Binding != c.Binding || a.Resource != c.Resource {
		return ErrScope
	}
	if a.SourceRevision != c.SourceRevision || a.ResourceRevision != c.ExpectedResourceRevision {
		return ErrRevision
	}
	if a.Provider != c.Provider {
		return ErrProvider
	}
	if a.Capability != coreruntime.Available {
		return ErrCapability
	}
	if !a.Owned {
		return ErrConflict
	}
	return nil
}

// PreviewBinding performs owner admission only. The returned values are not a
// lease or execution grant; ApplyBinding repeats every check under a fresh lease.
func PreviewBinding(ctx context.Context, c Candidate, owner Owner) (p Preview, err error) {
	if ctx == nil || owner == nil {
		return p, ErrLease
	}
	if err = validate(c); err != nil {
		return p, err
	}
	if err = ctx.Err(); err != nil {
		return p, err
	}
	c.Definition = slices.Clone(c.Definition)
	calls := 0
	_, err = owner.WithBindingLease(ctx, c, func(leased context.Context, l Lease) error {
		calls++
		if calls != 1 || leased == nil || l == nil {
			return ErrLease
		}
		if e := leased.Err(); e != nil {
			return e
		}
		if e := checkAdmission(c, l.Admission()); e != nil {
			return e
		}
		p = Preview{Admission: l.Admission(), CandidateDigest: Digest(c)}
		return nil
	})
	if calls != 1 && err == nil {
		err = ErrLease
	}
	if err != nil {
		p = Preview{}
	}
	return p, err
}

// ApplyBinding returns PENDING when a caller-owned transaction has not committed.
// APPLIED requires exact write observation and confirmed owner finalization.
// The owner retains rollback; uncertain writes are never replayed here.
func ApplyBinding(ctx context.Context, c Candidate, owner Owner) (r Receipt, err error) {
	r = Receipt{Binding: c.Binding, Resource: c.Resource, State: Failed}
	if ctx == nil || owner == nil {
		return r, ErrLease
	}
	if err = validate(c); err != nil {
		return r, err
	}
	if err = ctx.Err(); err != nil {
		return r, err
	}
	c.Definition = slices.Clone(c.Definition)
	digest := Digest(c)
	calls := 0
	final, err := owner.WithBindingLease(ctx, c, func(leased context.Context, l Lease) error {
		calls++
		if calls != 1 || leased == nil || l == nil {
			return ErrLease
		}
		if e := leased.Err(); e != nil {
			return e
		}
		admitted := l.Admission()
		if e := checkAdmission(c, admitted); e != nil {
			return e
		}
		if e := leased.Err(); e != nil {
			return e
		}
		r.ApplyAttempted = true
		r.State = Unknown
		if e := l.WriteBinding(leased, c); e != nil {
			return e
		}
		observed, e := l.ObserveBinding(leased)
		if e != nil {
			return ErrObservation
		}
		if e = leased.Err(); e != nil {
			return e
		}
		if l.Admission() != admitted || observed.Binding != c.Binding || observed.Provider != c.Provider || observed.SourceRevision != c.SourceRevision || observed.Resource.Host != c.Resource.Host || observed.Resource.Tag != c.Resource.Tag || (c.Resource.ID != 0 && observed.Resource.ID != c.Resource.ID) || !token(observed.ResourceRevision) || observed.CandidateDigest != digest {
			return ErrObservation
		}
		if c.Action == Remove {
			if observed.Present {
				return ErrObservation
			}
		} else if !observed.Present || observed.Resource.ID == 0 || observed.Missing != (c.Action == MarkMissing) {
			return ErrObservation
		}
		r.Resource = observed.Resource
		r.ResourceRevision = observed.ResourceRevision
		r.CandidateDigest = digest
		r.State = Pending
		return nil
	})
	if calls != 1 && err == nil {
		err = ErrLease
	}
	if err != nil {
		if r.ApplyAttempted && final != TransactionRolledBack {
			r.State = Unknown
		} else {
			r.State = Failed
		}
		return r, err
	}
	if r.State != Pending {
		return r, ErrLease
	}
	switch final {
	case TransactionCommitted:
		r.State = Applied
	case TransactionPending:
	case TransactionRolledBack:
		r.State = Failed
		err = ErrFinalization
	default:
		r.State = Unknown
		err = ErrFinalization
	}
	return r, err
}

// ObserveBinding reads only the explicit target. Missing or foreign Hosts never
// fall back to Local. The observation establishes no runtime activation proof.
func ObserveBinding(ctx context.Context, q Query, reader Reader) (Observation, error) {
	if ctx == nil || reader == nil || !validQuery(q) {
		return Observation{}, ErrScope
	}
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	o, err := reader.ObserveBinding(ctx, q)
	if err != nil {
		return Observation{}, ErrObservation
	}
	if err = ctx.Err(); err != nil {
		return Observation{}, err
	}
	if o.Binding != q.Binding || o.Resource.Host != q.Resource.Host || o.Resource.Tag != q.Resource.Tag || (q.Resource.ID != 0 && o.Resource.ID != q.Resource.ID) || !token(o.SourceRevision) || !token(o.ResourceRevision) {
		return Observation{}, ErrObservation
	}
	return o, nil
}
