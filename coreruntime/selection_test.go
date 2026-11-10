package coreruntime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

type selectionFixture struct {
	gate          chan struct{}
	admission     SelectionAdmission
	snapshot      SelectionSnapshot
	writes, reads int
	onSelect      func(context.Context, string, int) error
	onRead        func(int, *SelectionSnapshot) error
	finalizeErr   error
}

type selectionExecutorFunc func(context.Context, SelectionIntent, func(context.Context, SelectionLease) error) error

func (f selectionExecutorFunc) WithSelectionLease(ctx context.Context, i SelectionIntent, call func(context.Context, SelectionLease) error) error {
	return f(ctx, i, call)
}

func selectionTestFixture() (*selectionFixture, SelectionIntent) {
	host := hostref.Local()
	resource := ResourceRef{Host: host, Owner: SelectionOwner, Kind: SelectionKind, ID: "choice"}
	provider := Provider{ID: "sing-box", Revision: strings.Repeat("a", 64)}
	f := &selectionFixture{gate: make(chan struct{}, 1), admission: SelectionAdmission{Host: host, Resource: resource, Provider: provider, Revision: "generation-1", Capability: Available},
		snapshot: SelectionSnapshot{Host: host, Resource: resource, Provider: provider, Revision: "generation-1", Selectable: true, Selected: "direct", Members: []string{"direct", "second"}}}
	return f, SelectionIntent{Host: host, Resource: resource, Provider: provider, ExpectedRevision: "generation-1", Member: "second"}
}

func (f *selectionFixture) WithSelectionLease(ctx context.Context, _ SelectionIntent, call func(context.Context, SelectionLease) error) error {
	select {
	case f.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-f.gate }()
	if err := call(ctx, f); err != nil {
		return err
	}
	return f.finalizeErr
}
func (f *selectionFixture) Admission() SelectionAdmission { return f.admission }
func (f *selectionFixture) ReadSelection(ctx context.Context) (SelectionSnapshot, error) {
	f.reads++
	snapshot := f.snapshot
	snapshot.Members = slices.Clone(snapshot.Members)
	if f.onRead != nil {
		if err := f.onRead(f.reads, &snapshot); err != nil {
			return SelectionSnapshot{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return SelectionSnapshot{}, err
	}
	return snapshot, nil
}
func (f *selectionFixture) SelectMember(ctx context.Context, member string) error {
	f.writes++
	if f.onSelect != nil {
		return f.onSelect(ctx, member, f.writes)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f.snapshot.Selected = member
	return nil
}

func TestSelectionUsesObservedResultAndIdempotentRetry(t *testing.T) {
	for _, provider := range []string{"sing-box", "shtorm-extended"} {
		t.Run(provider, func(t *testing.T) {
			f, intent := selectionTestFixture()
			intent.Provider.ID = provider
			f.admission.Provider = intent.Provider
			f.snapshot.Provider = intent.Provider
			result, err := Select(context.Background(), intent, f)
			if err != nil || result.Outcome != SelectionVerified || result.Before != "direct" || result.Observed != "second" || result.Health.Proof != "RUNTIME_SELECTION_VERIFIED" || result.Health.ObservedAt.IsZero() {
				t.Fatalf("%+v %v", result, err)
			}
			again, err := Select(context.Background(), intent, f)
			if err != nil || again.Reason != "ALREADY_SELECTED" || again.ApplyAttempted || f.writes != 1 {
				t.Fatalf("retry dispatched twice: %+v %v", again, err)
			}
		})
	}
}

func TestSelectionRejectsForeignScopeStaleRevisionAndUnknownCapabilities(t *testing.T) {
	cases := map[string]struct {
		mutate func(*selectionFixture, *SelectionIntent)
		want   error
	}{
		"wrong-host":              {func(_ *selectionFixture, i *SelectionIntent) { i.Host.ID = "foreign" }, ErrSelectionScope},
		"foreign-resource":        {func(f *selectionFixture, _ *SelectionIntent) { f.admission.Resource.ID = "foreign" }, ErrSelectionScope},
		"foreign-owner":           {func(_ *selectionFixture, i *SelectionIntent) { i.Resource.Owner = "routing" }, ErrSelectionScope},
		"wrong-kind":              {func(_ *selectionFixture, i *SelectionIntent) { i.Resource.Kind = "firewall" }, ErrSelectionScope},
		"stale-revision":          {func(_ *selectionFixture, i *SelectionIntent) { i.ExpectedRevision = "generation-old" }, ErrSelectionRevision},
		"missing-revision":        {func(_ *selectionFixture, i *SelectionIntent) { i.ExpectedRevision = "" }, ErrSelectionScope},
		"third-provider":          {func(_ *selectionFixture, i *SelectionIntent) { i.Provider.ID = "third" }, ErrSelectionProvider},
		"provider-changed":        {func(f *selectionFixture, _ *SelectionIntent) { f.admission.Provider.ID = "shtorm-extended" }, ErrSelectionProvider},
		"provider-source-changed": {func(f *selectionFixture, _ *SelectionIntent) { f.admission.Provider.Revision = "other" }, ErrSelectionProvider},
		"capability-unknown":      {func(f *selectionFixture, _ *SelectionIntent) { f.admission.Capability = Unknown }, ErrSelectionCapability},
		"capability-unsupported":  {func(f *selectionFixture, _ *SelectionIntent) { f.admission.Capability = Unsupported }, ErrSelectionCapability},
		"not-selectable":          {func(f *selectionFixture, _ *SelectionIntent) { f.snapshot.Selectable = false }, ErrSelectionCapability},
		"unknown-member":          {func(_ *selectionFixture, i *SelectionIntent) { i.Member = "missing" }, ErrSelectionCapability},
		"observed-wrong-host":     {func(f *selectionFixture, _ *SelectionIntent) { f.snapshot.Host.ID = "foreign" }, ErrSelectionScope},
		"observed-wrong-revision": {func(f *selectionFixture, _ *SelectionIntent) { f.snapshot.Revision = "other" }, ErrSelectionRevision},
		"observed-wrong-provider": {func(f *selectionFixture, _ *SelectionIntent) { f.snapshot.Provider.ID = "shtorm-extended" }, ErrSelectionProvider},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			f, intent := selectionTestFixture()
			test.mutate(f, &intent)
			result, err := Select(context.Background(), intent, f)
			if !errors.Is(err, test.want) || result.Outcome != SelectionRejected || f.writes != 0 || result.Health.State != "UNKNOWN" {
				t.Fatalf("unsafe admission: %+v %v writes=%d", result, err, f.writes)
			}
		})
	}
}

func TestSelectionRecoversFailedVerificationAndAmbiguousApply(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(map[bool]string{false: "failed-verification", true: "ambiguous-apply"}[ambiguous], func(t *testing.T) {
			f, intent := selectionTestFixture()
			f.onSelect = func(_ context.Context, member string, attempt int) error {
				if attempt == 1 {
					if ambiguous {
						f.snapshot.Selected = member
						return errors.New("ack lost")
					}
					return nil
				}
				f.snapshot.Selected = member
				return nil
			}
			result, err := Select(context.Background(), intent, f)
			if err == nil || result.Outcome != SelectionRestored || result.Observed != "direct" || f.writes != 2 || !result.RecoveryAttempted || result.Health.Proof != "RUNTIME_SELECTION_RESTORED" {
				t.Fatalf("unverified recovery: %+v %v", result, err)
			}
		})
	}
}

func TestSelectionCancellationRecoversWithinOriginalLease(t *testing.T) {
	f, intent := selectionTestFixture()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.onSelect = func(ctx context.Context, member string, attempt int) error {
		if attempt == 1 {
			f.snapshot.Selected = member
			cancel()
			return ctx.Err()
		}
		if ctx.Err() != nil {
			t.Fatal("recovery reused cancelled request")
		}
		f.snapshot.Selected = member
		return nil
	}
	result, err := Select(ctx, intent, f)
	if !errors.Is(err, context.Canceled) || result.Outcome != SelectionRestored || f.snapshot.Selected != "direct" {
		t.Fatalf("%+v %v", result, err)
	}
	f, intent = selectionTestFixture()
	result, err = Select(ctx, intent, f)
	if !errors.Is(err, context.Canceled) || f.writes != 0 || result.ApplyAttempted {
		t.Fatal("cancelled request dispatched")
	}
}

func TestSelectionRollbackFailureRemainsAmbiguous(t *testing.T) {
	for _, failure := range []string{"rollback", "verification", "identity"} {
		t.Run(failure, func(t *testing.T) {
			f, intent := selectionTestFixture()
			f.onSelect = func(_ context.Context, member string, attempt int) error {
				f.snapshot.Selected = member
				if attempt == 1 {
					return errors.New("ack lost")
				}
				if failure == "rollback" {
					return errors.New("restore failed")
				}
				return nil
			}
			f.onRead = func(read int, s *SelectionSnapshot) error {
				if read >= 3 {
					if failure == "verification" {
						return errors.New("read failed")
					}
					if failure == "identity" {
						s.Revision = "replacement"
					}
				}
				return nil
			}
			result, err := Select(context.Background(), intent, f)
			if !errors.Is(err, ErrSelectionRecovery) || result.Outcome != SelectionAmbiguous || result.Health.State != "UNKNOWN" || result.Observed != "" {
				t.Fatalf("false recovery: %+v %v", result, err)
			}
		})
	}
}

func TestSelectionNeverRestoresAcrossChangedProviderOrGeneration(t *testing.T) {
	for _, identity := range []string{"provider", "revision", "resource"} {
		t.Run(identity, func(t *testing.T) {
			f, intent := selectionTestFixture()
			f.onRead = func(read int, s *SelectionSnapshot) error {
				if read == 2 {
					switch identity {
					case "provider":
						s.Provider.ID = "shtorm-extended"
					case "revision":
						s.Revision = "replacement"
					case "resource":
						s.Resource.ID = "foreign"
					}
				}
				return nil
			}
			result, err := Select(context.Background(), intent, f)
			if !errors.Is(err, ErrSelectionRecovery) || result.Outcome != SelectionAmbiguous || f.writes != 1 {
				t.Fatalf("cross-identity rollback: %+v %v writes=%d", result, err, f.writes)
			}
		})
	}
}

func TestSelectionLeaseExcludesConcurrentApplyAndHonorsWaitingCancellation(t *testing.T) {
	f, intent := selectionTestFixture()
	entered, release := make(chan struct{}), make(chan struct{})
	f.onSelect = func(ctx context.Context, member string, _ int) error {
		close(entered)
		<-release
		f.snapshot.Selected = member
		return ctx.Err()
	}
	done := make(chan error, 1)
	go func() { _, err := Select(context.Background(), intent, f); done <- err }()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Select(ctx, intent, f); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := Select(context.Background(), intent, f); err != nil || f.writes != 1 {
		t.Fatal("duplicate concurrent apply", err, f.writes)
	}
}

func TestSelectionLeaseFinalizationCannotHideAmbiguity(t *testing.T) {
	f, intent := selectionTestFixture()
	f.finalizeErr = errors.New("lease outcome lost")
	result, err := Select(context.Background(), intent, f)
	if err == nil || result.Outcome != SelectionAmbiguous || result.Health.State != "UNKNOWN" {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestSelectionRequiresOwnerAdmissionAndExactlyOneLease(t *testing.T) {
	for _, mode := range []string{"denied", "missing", "ignored-error", "repeated"} {
		t.Run(mode, func(t *testing.T) {
			f, intent := selectionTestFixture()
			executor := selectionExecutorFunc(func(ctx context.Context, i SelectionIntent, call func(context.Context, SelectionLease) error) error {
				switch mode {
				case "denied":
					return ErrSelectionScope
				case "missing":
					return nil
				case "ignored-error":
					f.admission.Capability = Unknown
					_ = call(ctx, f)
					return nil
				default:
					_ = call(ctx, f)
					_ = call(ctx, f)
					return nil
				}
			})
			result, err := Select(context.Background(), intent, executor)
			if err == nil || result.Outcome == SelectionVerified {
				t.Fatalf("owner protocol violation accepted: %+v %v", result, err)
			}
		})
	}
}

func TestSelectionConcurrentDuplicateWaitsForExistingOwnerLease(t *testing.T) {
	f, intent := selectionTestFixture()
	entered, release, waiting := make(chan struct{}), make(chan struct{}), make(chan struct{})
	f.onSelect = func(_ context.Context, member string, _ int) error {
		close(entered)
		<-release
		f.snapshot.Selected = member
		return nil
	}
	first := make(chan error, 1)
	go func() { _, err := Select(context.Background(), intent, f); first <- err }()
	<-entered
	waiter := selectionExecutorFunc(func(ctx context.Context, i SelectionIntent, call func(context.Context, SelectionLease) error) error {
		close(waiting)
		return f.WithSelectionLease(ctx, i, call)
	})
	second := make(chan error, 1)
	go func() { _, err := Select(context.Background(), intent, waiter); second <- err }()
	<-waiting
	select {
	case err := <-second:
		t.Fatalf("competing operation bypassed lease: %v", err)
	default:
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if f.writes != 1 {
		t.Fatal("duplicate apply", f.writes)
	}
}
