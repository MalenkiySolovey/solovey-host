package outbounds

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/MalenkiySolovey/solovey-host/coreruntime"
	"github.com/MalenkiySolovey/solovey-host/hostref"
)

type testOwner struct {
	admission   Admission
	final       Finalization
	writes      int
	calls       int
	before      func()
	write       func()
	writeErr    error
	observeErr  error
	finalizeErr error
	observation func(*Observation)
}

func candidate() Candidate {
	local := hostref.Local()
	return Candidate{Binding: BindingRef{local, "source-1", "endpoint-2"}, Resource: ResourceRef{local, 3, "source-node"}, Provider: coreruntime.Provider{ID: "sing-box", Revision: "provider-1"}, SourceRevision: "source-revision-1", ExpectedResourceRevision: "resource-revision-1", Action: Upsert, Definition: json.RawMessage(`{"type":"vless","tag":"source-node","server":"example.org","uuid":"private-canary"}`)}
}
func ownerFor(c Candidate) *testOwner {
	return &testOwner{admission: Admission{c.Binding, c.Resource, c.Provider, c.SourceRevision, c.ExpectedResourceRevision, coreruntime.Available, true}, final: TransactionCommitted, calls: 1}
}
func (o *testOwner) WithBindingLease(ctx context.Context, c Candidate, fn func(context.Context, Lease) error) (Finalization, error) {
	if o.before != nil {
		o.before()
	}
	for i := 0; i < o.calls; i++ {
		if err := fn(ctx, &testLease{o: o, c: c}); err != nil {
			return o.final, err
		}
	}
	return o.final, o.finalizeErr
}

type testLease struct {
	o *testOwner
	c Candidate
}

func (l *testLease) Admission() Admission { return l.o.admission }
func (l *testLease) WriteBinding(ctx context.Context, c Candidate) error {
	l.o.writes++
	if l.o.write != nil {
		l.o.write()
	}
	return l.o.writeErr
}
func (l *testLease) ObserveBinding(context.Context) (Observation, error) {
	o := Observation{Binding: l.c.Binding, Resource: l.c.Resource, Provider: l.c.Provider, SourceRevision: l.c.SourceRevision, ResourceRevision: "resource-revision-2", Present: l.c.Action != Remove, Missing: l.c.Action == MarkMissing, CandidateDigest: Digest(l.c)}
	if l.o.observation != nil {
		l.o.observation(&o)
	}
	return o, l.o.observeErr
}
func TestBindingPreviewAndFreshApplyAdmission(t *testing.T) {
	c := candidate()
	owner := ownerFor(c)
	p, err := PreviewBinding(context.Background(), c, owner)
	if err != nil || owner.writes != 0 || p.CandidateDigest == "" {
		t.Fatalf("preview=%+v err=%v writes=%d", p, err, owner.writes)
	}
	owner.admission.ResourceRevision = "changed-after-preview"
	r, err := ApplyBinding(context.Background(), c, owner)
	if !errors.Is(err, ErrRevision) || r.State != Failed || owner.writes != 0 {
		t.Fatalf("stale preview was authority: %+v %v", r, err)
	}
}
func TestBindingAdmissionRejectsForeignStaleUnknownAndConflictingWrites(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Candidate, *testOwner)
		want   error
	}{
		{"foreign requested Host", func(c *Candidate, o *testOwner) { c.Binding.Host = hostref.Ref{Kind: "NODE", ID: "remote"} }, ErrScope},
		{"unknown requested Host", func(c *Candidate, o *testOwner) { c.Binding.Host = hostref.Ref{} }, ErrScope},
		{"foreign resource", func(c *Candidate, o *testOwner) { c.Resource.Host.ID = "other" }, ErrScope},
		{"foreign owner admission", func(c *Candidate, o *testOwner) { o.admission.Binding.EndpointID = "another" }, ErrScope},
		{"source revision", func(c *Candidate, o *testOwner) { o.admission.SourceRevision = "later" }, ErrRevision},
		{"resource revision", func(c *Candidate, o *testOwner) { o.admission.ResourceRevision = "later" }, ErrRevision},
		{"provider replacement", func(c *Candidate, o *testOwner) { o.admission.Provider.Revision = "later" }, ErrProvider},
		{"unknown capability", func(c *Candidate, o *testOwner) { o.admission.Capability = coreruntime.Unknown }, ErrCapability},
		{"unsupported capability", func(c *Candidate, o *testOwner) { o.admission.Capability = coreruntime.Unsupported }, ErrCapability},
		{"missing capability", func(c *Candidate, o *testOwner) { o.admission.Capability = "" }, ErrCapability},
		{"writer conflict", func(c *Candidate, o *testOwner) { o.admission.Owned = false }, ErrConflict},
		{"zero revision", func(c *Candidate, o *testOwner) { c.ExpectedResourceRevision = "" }, ErrScope},
		{"global Core config", func(c *Candidate, o *testOwner) {
			c.Definition = json.RawMessage(`{"type":"vless","tag":"source-node","inbounds":[]}`)
		}, ErrDefinition},
		{"wrong definition tag", func(c *Candidate, o *testOwner) { c.Definition = json.RawMessage(`{"type":"vless","tag":"another"}`) }, ErrDefinition},
		{"oversized definition", func(c *Candidate, o *testOwner) {
			c.Definition = json.RawMessage(strings.Repeat(" ", MaxDefinitionBytes+1))
		}, ErrDefinition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := candidate()
			owner := ownerFor(c)
			tt.change(&c, owner)
			r, err := ApplyBinding(context.Background(), c, owner)
			if !errors.Is(err, tt.want) || r.State != Failed || r.ApplyAttempted || owner.writes != 0 {
				t.Fatalf("receipt=%+v err=%v writes=%d", r, err, owner.writes)
			}
			if strings.Contains(err.Error(), "private-canary") {
				t.Fatal("definition leaked")
			}
		})
	}
}
func TestBindingReceiptRequiresOwnerWriteAndTransactionFinalization(t *testing.T) {
	for _, action := range []Action{Upsert, MarkMissing, Remove} {
		for _, final := range []Finalization{TransactionCommitted, TransactionPending, TransactionRolledBack, TransactionUnknown} {
			t.Run(string(action)+"/"+string(final), func(t *testing.T) {
				c := candidate()
				c.Action = action
				if action != Upsert {
					c.Definition = nil
				}
				owner := ownerFor(c)
				owner.final = final
				r, err := ApplyBinding(context.Background(), c, owner)
				want := map[Finalization]State{TransactionCommitted: Applied, TransactionPending: Pending, TransactionRolledBack: Failed, TransactionUnknown: Unknown}[final]
				if r.State != want || !r.ApplyAttempted || owner.writes != 1 || r.CandidateDigest == "" {
					t.Fatalf("receipt=%+v err=%v writes=%d", r, err, owner.writes)
				}
				if (final == TransactionCommitted || final == TransactionPending) != (err == nil) {
					t.Fatalf("unexpected finalization error: %v", err)
				}
			})
		}
	}
}
func TestBindingCancellationAndAmbiguousEffectsAreNotReplayed(t *testing.T) {
	t.Run("before admission", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c := candidate()
		o := ownerFor(c)
		r, err := ApplyBinding(ctx, c, o)
		if !errors.Is(err, context.Canceled) || r.ApplyAttempted || o.writes != 0 {
			t.Fatalf("%+v %v", r, err)
		}
	})
	t.Run("after possible write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := candidate()
		o := ownerFor(c)
		o.write = cancel
		r, err := ApplyBinding(ctx, c, o)
		if !errors.Is(err, context.Canceled) || r.State != Unknown || o.writes != 1 {
			t.Fatalf("%+v %v", r, err)
		}
	})
	for _, rolledBack := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncertain owner error", true: "confirmed rollback"}[rolledBack], func(t *testing.T) {
			c := candidate()
			o := ownerFor(c)
			o.writeErr = errors.New("write interrupted")
			o.final = TransactionUnknown
			want := Unknown
			if rolledBack {
				o.final = TransactionRolledBack
				want = Failed
			}
			r, err := ApplyBinding(context.Background(), c, o)
			if err == nil || r.State != want || o.writes != 1 {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}
func TestBindingRejectsUnverifiedObservationAndFailedFinalization(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testOwner)
	}{
		{"no observation", func(o *testOwner) { o.observeErr = errors.New("offline") }},
		{"snapshot alone", func(o *testOwner) { o.observation = func(v *Observation) { v.CandidateDigest = "" } }},
		{"foreign observation", func(o *testOwner) { o.observation = func(v *Observation) { v.Resource.Host.ID = "foreign" } }},
		{"different resource", func(o *testOwner) { o.observation = func(v *Observation) { v.Resource.ID++ } }},
		{"source changed", func(o *testOwner) { o.observation = func(v *Observation) { v.SourceRevision = "changed" } }},
		{"provider changed", func(o *testOwner) { o.observation = func(v *Observation) { v.Provider.Revision = "changed" } }},
		{"lost resource", func(o *testOwner) { o.observation = func(v *Observation) { v.Present = false } }},
		{"changed lease", func(o *testOwner) { o.write = func() { o.admission.SourceRevision = "changed" } }},
		{"commit error", func(o *testOwner) { o.finalizeErr = errors.New("commit outcome unknown") }},
		{"duplicate callback", func(o *testOwner) { o.calls = 2 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := candidate()
			o := ownerFor(c)
			tt.setup(o)
			r, err := ApplyBinding(context.Background(), c, o)
			if err == nil || r.State != Unknown || o.writes != 1 {
				t.Fatalf("%+v %v writes=%d", r, err, o.writes)
			}
		})
	}
	c := candidate()
	o := ownerFor(c)
	o.calls = 0
	r, err := ApplyBinding(context.Background(), c, o)
	if !errors.Is(err, ErrLease) || r.State != Failed {
		t.Fatalf("missing lease: %+v %v", r, err)
	}
}
func TestBindingFreezesCallerDefinition(t *testing.T) {
	c := candidate()
	o := ownerFor(c)
	before := Digest(c)
	o.before = func() {
		for i := range c.Definition {
			c.Definition[i] = ' '
		}
	}
	r, err := ApplyBinding(context.Background(), c, o)
	if err != nil || r.State != Applied || r.CandidateDigest != before {
		t.Fatalf("caller changed admitted definition: %+v %v", r, err)
	}
}

type readerFunc func(context.Context, Query) (Observation, error)

func (f readerFunc) ObserveBinding(ctx context.Context, q Query) (Observation, error) {
	return f(ctx, q)
}
func TestBindingObservationNeverFallsBackToLocal(t *testing.T) {
	c := candidate()
	q := Query{c.Binding, c.Resource}
	calls := 0
	reader := readerFunc(func(ctx context.Context, q Query) (Observation, error) {
		calls++
		return Observation{Binding: q.Binding, Resource: q.Resource, SourceRevision: "source-rev", ResourceRevision: "resource-rev", Present: true}, nil
	})
	if _, err := ObserveBinding(context.Background(), q, reader); err != nil {
		t.Fatal(err)
	}
	q.Binding.Host = hostref.Ref{Kind: "NODE", ID: "missing"}
	if _, err := ObserveBinding(context.Background(), q, reader); !errors.Is(err, ErrScope) || calls != 1 {
		t.Fatal("foreign read fell back to Local")
	}
}

func TestBindingPreservesNativeLimiterResourceRoute(t *testing.T) {
	for _, kind := range []string{"bandwidth-limiter", "connection-limiter", "rate-limiter", "traffic-limiter"} {
		t.Run(kind, func(t *testing.T) {
			c := candidate()
			c.Definition = json.RawMessage(`{"type":"` + kind + `","tag":"source-node","route":{"final":"member","rules":[{"outbound":"member"}]}}`)
			o := ownerFor(c)
			r, err := ApplyBinding(context.Background(), c, o)
			if err != nil || r.State != Applied || o.writes != 1 {
				t.Fatalf("native limiter projection rejected: %+v %v", r, err)
			}
		})
	}
}
