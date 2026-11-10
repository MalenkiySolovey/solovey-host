// Package coreruntime coordinates the existing owner-specific runtime group
// selection. Provider APIs, authorization and operation leases stay injected.
package coreruntime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

const SelectionOwner = "core-runtime"
const SelectionKind = "outbound-group"

type ResourceRef struct {
	Host  hostref.Ref
	Owner string
	Kind  string
	ID    string
}

type Provider struct{ ID, Revision string }

type Capability string

const (
	Available   Capability = "AVAILABLE"
	Unsupported Capability = "UNSUPPORTED"
	Unknown     Capability = "UNKNOWN"
)

// ExpectedRevision is the existing runtime generation, not a Desired revision.
// Selection does not change the generation or write configuration.
type SelectionIntent struct {
	Host             hostref.Ref
	Resource         ResourceRef
	Provider         Provider
	ExpectedRevision string
	Member           string
}

// SelectionAdmission comes from the accepted owner after transport
// authorization and while holding its operation/lifecycle lease. A caller's
// intent cannot supply this authority. The injected owner is trusted code.
type SelectionAdmission struct {
	Host       hostref.Ref
	Resource   ResourceRef
	Provider   Provider
	Revision   string
	Capability Capability
}

type SelectionSnapshot struct {
	Host       hostref.Ref
	Resource   ResourceRef
	Provider   Provider
	Revision   string
	Selectable bool
	Selected   string
	Members    []string
}

// SelectionLease exposes only this admitted resource. It must fence provider
// replacement and competing selections through verification and recovery.
type SelectionLease interface {
	Admission() SelectionAdmission
	ReadSelection(context.Context) (SelectionSnapshot, error)
	SelectMember(context.Context, string) error
}

type SelectionExecutor interface {
	WithSelectionLease(context.Context, SelectionIntent, func(context.Context, SelectionLease) error) error
}

type SelectionOutcome string

const (
	SelectionRejected  SelectionOutcome = "REJECTED"
	SelectionVerified  SelectionOutcome = "SUCCESS"
	SelectionRestored  SelectionOutcome = "ROLLED_BACK"
	SelectionAmbiguous SelectionOutcome = "AMBIGUOUS"
)

// SelectionHealth proves one runtime selection observation only. It grants no
// data-plane, end-to-end, artifact-trust or durable Desired-state assertion.
type SelectionHealth struct {
	State      string
	Proof      string
	Source     string
	ObservedAt time.Time
}

type SelectionResult struct {
	Host              hostref.Ref
	Resource          ResourceRef
	Provider          Provider
	ExpectedRevision  string
	Requested         string
	Before            string
	Observed          string
	Outcome           SelectionOutcome
	Reason            string
	ApplyAttempted    bool
	RecoveryAttempted bool
	Health            SelectionHealth
}

var (
	ErrSelectionScope        = errors.New("runtime selection scope rejected")
	ErrSelectionRevision     = errors.New("runtime selection revision changed")
	ErrSelectionProvider     = errors.New("runtime selection provider incompatible")
	ErrSelectionCapability   = errors.New("runtime selection capability unavailable")
	ErrSelectionUnavailable  = errors.New("runtime selection observation unavailable")
	ErrSelectionVerification = errors.New("runtime selection verification failed")
	ErrSelectionRecovery     = errors.New("runtime selection recovery unverified")
	ErrSelectionLease        = errors.New("runtime selection lease unavailable")
)

const selectionTimeout = 3 * time.Second

func validTag(s string) bool { return s != "" && len(s) <= 256 && !strings.ContainsAny(s, "\x00\r\n") }
func validProvider(p Provider) bool {
	return (p.ID == "sing-box" || p.ID == "shtorm-extended") && validTag(p.Revision)
}
func validIntent(i SelectionIntent) bool {
	return i.Host.IsLocal() && i.Resource.Host == i.Host && i.Resource.Owner == SelectionOwner &&
		i.Resource.Kind == SelectionKind && validTag(i.Resource.ID) && validTag(i.ExpectedRevision) && validTag(i.Member)
}

// Select executes only an already authorized CoreRuntime owner intent. The
// executor must call the callback exactly once under one continuous lease.
// No worker, authorization service, registry or storage is created here.
func Select(ctx context.Context, intent SelectionIntent, executor SelectionExecutor) (result SelectionResult, err error) {
	result = SelectionResult{Host: intent.Host, Resource: intent.Resource, Provider: intent.Provider,
		ExpectedRevision: intent.ExpectedRevision, Requested: intent.Member, Outcome: SelectionRejected,
		Health: SelectionHealth{State: "UNKNOWN"}}
	if ctx == nil || executor == nil || !validIntent(intent) {
		return result, ErrSelectionScope
	}
	if !validProvider(intent.Provider) {
		return result, ErrSelectionProvider
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	bounded, cancel := context.WithTimeout(ctx, selectionTimeout)
	defer cancel()
	calls := 0
	err = executor.WithSelectionLease(bounded, intent, func(leased context.Context, lease SelectionLease) error {
		calls++
		if calls != 1 || leased == nil || lease == nil {
			return ErrSelectionLease
		}
		if err := leased.Err(); err != nil {
			return err
		}
		admitted := lease.Admission()
		if admitted.Host != intent.Host || admitted.Resource != intent.Resource {
			return ErrSelectionScope
		}
		if admitted.Revision != intent.ExpectedRevision {
			return ErrSelectionRevision
		}
		if admitted.Provider != intent.Provider {
			return ErrSelectionProvider
		}
		if admitted.Capability != Available {
			return ErrSelectionCapability
		}
		before, err := lease.ReadSelection(leased)
		if err != nil {
			return ErrSelectionUnavailable
		}
		if err := checkSelectionSnapshot(intent, before); err != nil {
			return err
		}
		if !before.Selectable || !slices.Contains(before.Members, intent.Member) || !slices.Contains(before.Members, before.Selected) {
			return ErrSelectionCapability
		}
		result.Before, result.Observed = before.Selected, before.Selected
		if err := leased.Err(); err != nil {
			return err
		}
		if before.Selected == intent.Member {
			result.Outcome, result.Reason = SelectionVerified, "ALREADY_SELECTED"
			result.Health = selectionHealth("RUNTIME_SELECTION_VERIFIED")
			return nil
		}
		result.ApplyAttempted = true
		applyErr := lease.SelectMember(leased, intent.Member)
		observed, readErr := lease.ReadSelection(leased)
		if applyErr == nil && readErr == nil && checkSelectionSnapshot(intent, observed) == nil && observed.Selectable &&
			observed.Selected == intent.Member && slices.Contains(observed.Members, intent.Member) && leased.Err() == nil {
			result.Observed, result.Outcome, result.Reason = observed.Selected, SelectionVerified, "SELECTION_VERIFIED"
			result.Health = selectionHealth("RUNTIME_SELECTION_VERIFIED")
			return nil
		}
		cause := error(ErrSelectionVerification)
		if applyErr != nil {
			cause = ErrSelectionUnavailable
		}
		if leased.Err() != nil {
			cause = leased.Err()
		}
		result.RecoveryAttempted = true
		result.Outcome, result.Reason, result.Observed = SelectionAmbiguous, "RECOVERY_UNVERIFIED", ""
		// Never restore across a changed scope, generation or provider. The
		// lease is the authority even when the provider read itself failed.
		if lease.Admission() != admitted || readErr == nil && (observed.Host != intent.Host || observed.Resource != intent.Resource ||
			observed.Provider != intent.Provider || observed.Revision != intent.ExpectedRevision) {
			return errors.Join(cause, ErrSelectionRecovery)
		}
		// Keep the same owner lease; remove only the cancelled request deadline.
		// Recovery is independently bounded, never a detached background job.
		recovery, stop := context.WithTimeout(context.WithoutCancel(leased), selectionTimeout)
		defer stop()
		restoreErr := lease.SelectMember(recovery, before.Selected)
		restored, verifyErr := lease.ReadSelection(recovery)
		if restoreErr == nil && verifyErr == nil && checkSelectionSnapshot(intent, restored) == nil && restored.Selectable &&
			restored.Selected == before.Selected && slices.Contains(restored.Members, before.Selected) && recovery.Err() == nil {
			result.Observed, result.Outcome, result.Reason = restored.Selected, SelectionRestored, "BEFORE_IMAGE_RESTORED"
			result.Health = selectionHealth("RUNTIME_SELECTION_RESTORED")
			return cause
		}
		return errors.Join(cause, ErrSelectionRecovery)
	})
	if (calls != 1 || result.Outcome != SelectionVerified) && err == nil {
		err = ErrSelectionLease
	}
	if err != nil && result.Outcome == SelectionVerified {
		// A failed lease finalization cannot turn into unconditional success.
		result.Outcome, result.Reason, result.Health = SelectionAmbiguous, "LEASE_RESULT_UNVERIFIED", SelectionHealth{State: "UNKNOWN"}
	}
	if result.Reason == "" {
		result.Reason = "ADMISSION_OR_OBSERVATION_REJECTED"
	}
	return result, err
}

func checkSelectionSnapshot(i SelectionIntent, s SelectionSnapshot) error {
	if s.Host != i.Host || s.Resource != i.Resource {
		return ErrSelectionScope
	}
	if s.Revision != i.ExpectedRevision {
		return ErrSelectionRevision
	}
	if s.Provider != i.Provider {
		return ErrSelectionProvider
	}
	if len(s.Members) > 256 || !validTag(s.Selected) {
		return ErrSelectionUnavailable
	}
	for _, member := range s.Members {
		if !validTag(member) {
			return ErrSelectionUnavailable
		}
	}
	return nil
}

func selectionHealth(proof string) SelectionHealth {
	return SelectionHealth{State: "HEALTHY", Proof: proof, Source: "provider_api", ObservedAt: time.Now().UTC()}
}
