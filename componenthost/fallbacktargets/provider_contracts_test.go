package fallbacktargets

import (
	"encoding/json"
	hostresources "github.com/MalenkiySolovey/solovey-host/componenthost/resources"
	"strings"
	"testing"
	"time"
)

func TestExpiredCapacityIsExplicitlyStale(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	target := validTargetV2(t, "provider", "target", "endpoint", now)
	if got := EffectiveCapacityStateV2(target.Capacity, now.Add(2*time.Minute)); got != CapacityState("STALE") {
		t.Fatalf("expired capacity collapsed to %q instead of STALE", got)
	}
}

func TestFallbackTargetReferenceV2ExactAndStale(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	target := validTargetV2(t, "provider", "target", "endpoint", now)
	reference, err := ReferenceV2FromTarget(target)
	if err != nil || ResolveExactV2(reference, target, now) != nil {
		t.Fatalf("exact resolution failed: %v", err)
	}
	mutations := map[string]func(*FallbackTargetReferenceV2){
		"publish":  func(v *FallbackTargetReferenceV2) { v.PublishRevision = "publish-2" },
		"digest":   func(v *FallbackTargetReferenceV2) { v.ContentDigest = strings.Repeat("b", 64) },
		"endpoint": func(v *FallbackTargetReferenceV2) { v.EndpointRevision = strings.Repeat("b", 64) },
		"health":   func(v *FallbackTargetReferenceV2) { v.ProviderHealthRevision = strings.Repeat("b", 64) },
		"capacity": func(v *FallbackTargetReferenceV2) { v.CapacityRevision = strings.Repeat("b", 64) },
		"provider": func(v *FallbackTargetReferenceV2) { v.ProviderRevision = "provider-2" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			stale := reference
			mutate(&stale)
			if ResolveExactV2(stale, target, now) == nil {
				t.Fatal("stale reference resolved")
			}
		})
	}
	partial := reference
	partial.CapacityRevision = ""
	if partial.Validate() == nil || (FallbackTargetReferenceV2{ProviderID: "provider", TargetID: "target", EndpointID: "endpoint"}).Validate() == nil {
		t.Fatal("partial or host/port-style reference became actionable")
	}
}

func TestFallbackTargetV2BoundsAndRedaction(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	base := validTargetV2(t, "provider", "target", "endpoint", now)
	for name, mutate := range map[string]func(*FallbackTargetV2){
		"protocols": func(v *FallbackTargetV2) {
			v.Endpoint.ApplicationProtocols = make([]ApplicationProtocol, MaxApplicationProtocolsV2+1)
		},
		"server-names":     func(v *FallbackTargetV2) { v.Endpoint.AcceptedServerNames = make([]string, MaxAcceptedServerNamesV2+1) },
		"health-reasons":   func(v *FallbackTargetV2) { v.Health.ReasonCodes = make([]string, MaxReasonCodesV2+1) },
		"capacity-reasons": func(v *FallbackTargetV2) { v.Capacity.ReasonCodes = make([]string, MaxReasonCodesV2+1) },
		"path":             func(v *FallbackTargetV2) { v.Source = "/var/lib/private/key.pem" },
		"url":              func(v *FallbackTargetV2) { v.Identity.TargetID = "https://secret.invalid" },
		"control":          func(v *FallbackTargetV2) { v.Source = "secret\nvalue" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.CanonicalTargetRevision, candidate.Endpoint.EndpointRevision, candidate.Health.Revision, candidate.Capacity.Revision = "", "", "", ""
			mutate(&candidate)
			_, err := FinalizeFallbackTargetV2(candidate)
			if err == nil {
				t.Fatal("unsafe or unbounded field accepted")
			}
			if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret.invalid") || strings.Contains(err.Error(), "secret\nvalue") {
				t.Fatalf("error leaked provider value: %v", err)
			}
		})
	}
	payload, err := json.Marshal(base)
	if err != nil || strings.Contains(string(payload), "key.pem") || strings.Contains(string(payload), "https://") {
		t.Fatalf("outward target leaked forbidden material: %s err=%v", payload, err)
	}
}

func TestFallbackTargetV2HealthAndCapacityStates(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	target := validTargetV2(t, "provider", "target", "endpoint", now)
	if EffectiveReadinessV2(target.Health, now) != ReadinessReady || EffectiveReadinessV2(target.Health, now.Add(2*time.Minute)) != ReadinessStale {
		t.Fatal("health freshness not represented")
	}
	unknownHealth := target.Health
	unknownHealth.Readiness = ReadinessUnknown
	if EffectiveReadinessV2(unknownHealth, now) != ReadinessUnknown {
		t.Fatal("unknown health fabricated")
	}
	for _, state := range []CapacityState{CapacityReady, CapacityPressured, CapacityExhausted, CapacityUnknown, CapacityStale} {
		capacity := target.Capacity
		capacity.State = state
		if state == CapacityExhausted {
			capacity.ReservationSlotsUsed = capacity.ReservationSlotsTotal
		}
		if got := EffectiveCapacityStateV2(capacity, now); got != state {
			t.Fatalf("capacity state %s became %s", state, got)
		}
	}
	if EffectiveCapacityStateV2(target.Capacity, now.Add(2*time.Minute)) != CapacityStale {
		t.Fatal("stale capacity remained actionable")
	}
	invalid := target
	invalid.CanonicalTargetRevision, invalid.Endpoint.EndpointRevision, invalid.Health.Revision, invalid.Capacity.Revision = "", "", "", ""
	invalid.Capacity.ReservationSlotsUsed = invalid.Capacity.ReservationSlotsTotal + 1
	if _, err := FinalizeFallbackTargetV2(invalid); err == nil {
		t.Fatal("contradictory capacity accepted")
	}
}

func TestFallbackTargetV2RejectsForgedCanonicalRevision(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	target := validTargetV2(t, "provider", "target", "endpoint", now)
	target.CanonicalTargetRevision = strings.Repeat("b", 64)
	if err := target.Validate(); err == nil {
		t.Fatal("forged canonical target revision accepted")
	}
}

func TestFallbackTargetV2RejectsMissingRevisionsAndInvalidEndpointFacts(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	base := validTargetV2(t, "provider", "target", "endpoint", now)
	withoutRevision := base
	withoutRevision.Health.Revision = ""
	if withoutRevision.Validate() == nil {
		t.Fatal("missing health revision accepted")
	}
	tests := map[string]func(*FallbackTargetV2){
		"network":    func(value *FallbackTargetV2) { value.Endpoint.Network = hostresources.NetworkUDP },
		"family":     func(value *FallbackTargetV2) { value.Endpoint.AddressFamily = hostresources.AddressFamilyIPv6 },
		"address":    func(value *FallbackTargetV2) { value.Endpoint.Address = "0.0.0.0" },
		"port":       func(value *FallbackTargetV2) { value.Endpoint.Port = 0 },
		"transport":  func(value *FallbackTargetV2) { value.Endpoint.TransportSecurity = "INVALID" },
		"management": func(value *FallbackTargetV2) { value.Endpoint.CanReachManagement = hostresources.CapabilityUnknown },
		"proxy":      func(value *FallbackTargetV2) { value.Endpoint.ProxyProtocol = hostresources.CapabilityYes },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.CanonicalTargetRevision, candidate.Endpoint.EndpointRevision, candidate.Health.Revision, candidate.Capacity.Revision = "", "", "", ""
			mutate(&candidate)
			if _, err := FinalizeFallbackTargetV2(candidate); err == nil {
				t.Fatal("invalid target finalized")
			}
		})
	}
}

func TestFallbackTargetV2RejectsUnboundedOrFutureObservations(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	base := validTargetV2(t, "provider", "target", "endpoint", now)
	tooLarge := base
	tooLarge.CanonicalTargetRevision, tooLarge.Endpoint.EndpointRevision, tooLarge.Health.Revision, tooLarge.Capacity.Revision = "", "", "", ""
	tooLarge.Capacity.ReservationSlotsTotal = MaxReservationsV2 + 1
	if _, err := FinalizeFallbackTargetV2(tooLarge); err == nil {
		t.Fatal("capacity above the neutral reservation bound accepted")
	}
	tooLong := base
	tooLong.CanonicalTargetRevision, tooLong.Endpoint.EndpointRevision, tooLong.Health.Revision, tooLong.Capacity.Revision = "", "", "", ""
	tooLong.Health.ExpiresAt = tooLong.Health.ObservedAt + int64((6*time.Minute)/time.Second)
	if _, err := FinalizeFallbackTargetV2(tooLong); err == nil {
		t.Fatal("unbounded health freshness accepted")
	}
	future := base
	future.CanonicalTargetRevision, future.Endpoint.EndpointRevision, future.Health.Revision, future.Capacity.Revision = "", "", "", ""
	future.Health.ObservedAt = now.Add(10 * time.Minute).Unix()
	future.Health.ExpiresAt = now.Add(11 * time.Minute).Unix()
	future.Capacity.ObservedAt = now.Add(10 * time.Minute).Unix()
	future.Capacity.ExpiresAt = now.Add(11 * time.Minute).Unix()
	future, err := FinalizeFallbackTargetV2(future)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := ReferenceV2FromTarget(future)
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveExactV2(reference, future, now); err == nil {
		t.Fatal("future observation resolved as currently actionable")
	}
}

func TestFallbackTargetV2RevisionCoversRelevantFields(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	base := validTargetV2(t, "provider", "target", "endpoint", now)
	mutations := []func(*FallbackTargetV2){
		func(v *FallbackTargetV2) { v.Publish.Revision = "publish-2" },
		func(v *FallbackTargetV2) { v.Publish.ContentDigest = strings.Repeat("b", 64) },
		func(v *FallbackTargetV2) { v.Endpoint.Port++ },
		func(v *FallbackTargetV2) { v.Endpoint.TransportSecurity = TransportSecurityPlaintext },
		func(v *FallbackTargetV2) {
			v.Endpoint.ApplicationProtocols = []ApplicationProtocol{ApplicationProtocolHTTP11}
		},
		func(v *FallbackTargetV2) { v.Endpoint.AcceptedServerNames = []string{"other.example.com"} },
		func(v *FallbackTargetV2) { v.Health.ExpiresAt++ },
		func(v *FallbackTargetV2) { v.Capacity.ReservationSlotsUsed++ },
		func(v *FallbackTargetV2) { v.ProviderRevision = "provider-2" },
		func(v *FallbackTargetV2) { v.Source = "provider_probe" },
		func(v *FallbackTargetV2) { v.ConfidenceBP-- },
	}
	for index, mutate := range mutations {
		candidate := base
		candidate.CanonicalTargetRevision, candidate.Endpoint.EndpointRevision, candidate.Health.Revision, candidate.Capacity.Revision = "", "", "", ""
		mutate(&candidate)
		finalized, err := FinalizeFallbackTargetV2(candidate)
		if err != nil {
			t.Fatalf("mutation %d: %v", index, err)
		}
		if finalized.CanonicalTargetRevision == base.CanonicalTargetRevision {
			t.Fatalf("mutation %d did not change target revision", index)
		}
	}
}

func TestFallbackTargetV2ValidDeterministicAndCanonical(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	first := validTargetV2(t, "provider-b", "target-b", "endpoint-b", now)
	if err := first.Validate(); err != nil {
		t.Fatal(err)
	}
	input := first
	input.CanonicalTargetRevision = ""
	input.Endpoint.EndpointRevision = ""
	input.Health.Revision = ""
	input.Capacity.Revision = ""
	slices := []struct {
		protocols                             []ApplicationProtocol
		names, healthReasons, capacityReasons []string
	}{
		{[]ApplicationProtocol{ApplicationProtocolHTTP11, ApplicationProtocolHTTP2}, []string{"example.com", "www.example.com"}, nil, nil},
		{[]ApplicationProtocol{ApplicationProtocolHTTP2, ApplicationProtocolHTTP11}, []string{"www.example.com", "example.com"}, []string{}, []string{}},
	}
	var revisions []string
	for _, order := range slices {
		candidate := input
		candidate.Endpoint.ApplicationProtocols = order.protocols
		candidate.Endpoint.AcceptedServerNames = order.names
		candidate.Health.ReasonCodes = order.healthReasons
		candidate.Capacity.ReasonCodes = order.capacityReasons
		finalized, err := FinalizeFallbackTargetV2(candidate)
		if err != nil {
			t.Fatal(err)
		}
		revisions = append(revisions, finalized.CanonicalTargetRevision)
	}
	if revisions[0] != revisions[1] {
		t.Fatalf("canonical revisions differ: %v", revisions)
	}
}

func TestProviderRequestsAreBoundedCASOnlyAndProviderAuthoritative(t *testing.T) {
	if (ReserveRequestV1{RequestID: "request", HolderID: "holder", Purpose: ReservationPurposeNativeFallback, FreshnessDurationSecs: 60}).Validate() == nil {
		t.Fatal("reserve without exact reference accepted")
	}
	if (ReservationMutationRequestV1{RequestID: "request", ReservationID: "reservation", ExpectedRevision: "revision", FreshnessDurationSecs: 60}).Validate(false) == nil {
		t.Fatal("caller supplied mutation freshness where not allowed")
	}
	if (ReleaseReservationRequestV1{RequestID: "request", ReservationID: "reservation", ExpectedRevision: "revision"}).Validate() == nil {
		t.Fatal("consumer mirror could release without provider-verified detachment")
	}
	if (ListReservationsQueryV1{Limit: MaxReservationListPageV2 + 1}).Validate() == nil || MaxReservationsV2 != 4096 || MaxTargetsV2 != 4096 {
		t.Fatal("list or global bounds changed")
	}
}

func TestProviderReservationAcceptsTypedFrontingPurposeWithoutWeakeningCAS(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	reservation := reservationFixture(t, now, ReservationReserved)
	reservation.Purpose = ReservationPurposeFronting
	if err := reservation.ValidateAt(now); err != nil {
		t.Fatal(err)
	}
	request := ReserveRequestV1{RequestID: "fronting-request", HolderID: reservation.HolderID, Purpose: ReservationPurposeFronting,
		ExactTargetReference: reservation.ExactTargetReference, FreshnessDurationSecs: 60}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	cas := ReservationCASV1{RequestID: "fronting-request", ReservationID: reservation.ReservationID, ExpectedRevision: "stale"}
	if ValidateReservationCAS(reservation, cas) == nil {
		t.Fatal("fronting purpose weakened provider CAS")
	}
}

func TestProviderReservationLegalTransitionMatrixAndCAS(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	paths := []struct {
		from, to ReservationState
		mutation ReservationMutationKind
	}{
		{ReservationReserved, ReservationMutationPending, ReservationMutationFence},
		{ReservationMutationPending, ReservationActive, ReservationMutationActivate},
		{ReservationActive, ReservationReleased, ReservationMutationRelease},
		{ReservationReserved, ReservationReleased, ReservationMutationRelease},
		{ReservationReconcileRequired, ReservationReleased, ReservationMutationRelease},
	}
	for _, path := range paths {
		current := reservationFixture(t, now, path.from)
		next := transitionedReservation(current, path.to)
		cas := ReservationCASV1{RequestID: "request-1", ReservationID: current.ReservationID, ExpectedRevision: current.ReservationRevision}
		validate := func(cas ReservationCASV1) error {
			if path.mutation == ReservationMutationRelease {
				return ValidateReservationReleaseTransition(current, next, ReleaseReservationRequestV1{
					RequestID: cas.RequestID, ReservationID: cas.ReservationID, ExpectedRevision: cas.ExpectedRevision,
					VerifiedDetachedRevision: strings.Repeat("d", 64),
				}, now)
			}
			return ValidateReservationTransition(current, next, cas, path.mutation, now)
		}
		if err := validate(cas); err != nil {
			t.Errorf("%s -> %s: %v", path.from, path.to, err)
		}
		cas.ExpectedRevision = "stale-revision"
		if validate(cas) == nil {
			t.Errorf("%s -> %s accepted stale CAS", path.from, path.to)
		}
	}
	for _, state := range []ReservationState{ReservationReserved, ReservationMutationPending, ReservationActive} {
		current := reservationFixture(t, now, state)
		next := transitionedReservation(current, ReservationReconcileRequired)
		cas := ReservationCASV1{RequestID: "request-1", ReservationID: current.ReservationID, ExpectedRevision: current.ReservationRevision}
		if err := ValidateReconcileTransition(current, next, cas, now); err != nil {
			t.Errorf("%s -> reconcile: %v", state, err)
		}
	}
}

func TestProviderReservationRejectsIllegalTransitionsAndReleasedIsTerminal(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	current := reservationFixture(t, now, ReservationMutationPending)
	cas := ReservationCASV1{RequestID: "request-1", ReservationID: current.ReservationID, ExpectedRevision: current.ReservationRevision}
	if ValidateReservationReleaseTransition(current, transitionedReservation(current, ReservationReleased), ReleaseReservationRequestV1{
		RequestID: "request-1", ReservationID: current.ReservationID, ExpectedRevision: current.ReservationRevision,
		VerifiedDetachedRevision: strings.Repeat("d", 64),
	}, now) != nil {
		t.Fatal("mutation-pending release with detachment proof was rejected")
	}
	released := reservationFixture(t, now, ReservationReleased)
	cas.ExpectedRevision = released.ReservationRevision
	if ValidateReservationReleaseTransition(released, transitionedReservation(released, ReservationReleased), ReleaseReservationRequestV1{
		RequestID: cas.RequestID, ReservationID: released.ReservationID, ExpectedRevision: released.ReservationRevision,
		VerifiedDetachedRevision: strings.Repeat("d", 64),
	}, now) == nil {
		t.Fatal("released reservation was not terminal")
	}
	conflict := transitionedReservation(reservationFixture(t, now, ReservationReserved), ReservationMutationPending)
	conflict.ReservationRevision = "revision-1"
	current = reservationFixture(t, now, ReservationReserved)
	cas = ReservationCASV1{RequestID: "request-2", ReservationID: current.ReservationID, ExpectedRevision: current.ReservationRevision}
	if ValidateReservationTransition(current, conflict, cas, ReservationMutationFence, now) == nil {
		t.Fatal("caller-selected unchanged next revision accepted")
	}
}

func TestProviderReservationReplayCapacityAndExpiry(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	reservation := reservationFixture(t, now, ReservationActive)
	if ValidateIdempotentReservationReplay(reservation, reservation) != nil {
		t.Fatal("identical replay rejected")
	}
	conflict := reservation
	conflict.HolderID = "operation-2"
	if ValidateIdempotentReservationReplay(reservation, conflict) == nil || ValidateReplayRequest("request-1", strings.Repeat("a", 64), strings.Repeat("b", 64), reservation, reservation) == nil {
		t.Fatal("conflicting replay accepted")
	}
	target := validTargetV2(t, "provider", "target", "endpoint", now)
	if ValidateReservationCapacity(target, 1, now) != nil || ValidateReservationCapacity(target, 8, now) == nil {
		t.Fatal("capacity limit was not enforced")
	}
	pressured := target
	pressured.CanonicalTargetRevision, pressured.Endpoint.EndpointRevision, pressured.Health.Revision, pressured.Capacity.Revision = "", "", "", ""
	pressured.Capacity.State = CapacityPressured
	pressured, err := FinalizeFallbackTargetV2(pressured)
	if err != nil || ValidateReservationCapacity(pressured, 1, now) == nil {
		t.Fatal("pressured target accepted a new reservation")
	}
	for state, wantBlock := range map[ReservationState]bool{
		ReservationReserved: false, ReservationMutationPending: true, ReservationActive: true,
		ReservationReconcileRequired: true, ReservationReleased: false,
	} {
		value := reservationFixture(t, now, state)
		status := value.Status(now.Add(10 * time.Minute))
		if status.BlocksMutation != wantBlock {
			t.Errorf("expired %s blocks=%v want=%v", state, status.BlocksMutation, wantBlock)
		}
		if (state == ReservationMutationPending || state == ReservationActive) && status.EffectiveState != ReservationReconcileRequired {
			t.Errorf("expired %s did not require reconciliation", state)
		}
	}
}

func TestReservationFreshnessBounds(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	reference, err := ReferenceV2FromTarget(validTargetV2(t, "provider", "target", "endpoint", now))
	if err != nil {
		t.Fatal(err)
	}
	request := ReserveRequestV1{RequestID: "request", HolderID: "holder", Purpose: ReservationPurposeNativeFallback, ExactTargetReference: reference, FreshnessDurationSecs: uint32((6 * time.Minute) / time.Second)}
	if err := request.Validate(); err == nil {
		t.Fatal("prepare reservation longer than five minutes accepted")
	}
	reservation := reservationFixture(t, now, ReservationActive)
	reservation.FreshnessExpiresAt = reservation.RenewedAt + int64((16*time.Minute)/time.Second)
	if err := reservation.Validate(); err == nil {
		t.Fatal("ACTIVE reservation longer than the bounded window accepted")
	}
	future := reservationFixture(t, now, ReservationActive)
	future.IssuedAt = now.Add(10 * time.Minute).Unix()
	future.RenewedAt = future.IssuedAt
	future.FreshnessExpiresAt = future.RenewedAt + int64(time.Minute/time.Second)
	if status := future.Status(now); status.EffectiveState != ReservationReconcileRequired || !status.BlocksMutation {
		t.Fatalf("future-dated authority became actionable: %#v", status)
	}
}

func TestReservationReplayCanonicalizesReasonOrdering(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	previous := reservationFixture(t, now, ReservationActive)
	previous.ReasonCodes = []string{"reason-b", "reason-a"}
	replay := previous
	replay.ReasonCodes = []string{"reason-a", "reason-b"}
	if err := ValidateIdempotentReservationReplay(previous, replay); err != nil {
		t.Fatalf("semantic replay rejected because of reason ordering: %v", err)
	}
	previous.ReasonCodes = nil
	replay.ReasonCodes = []string{}
	if err := ValidateIdempotentReservationReplay(previous, replay); err != nil {
		t.Fatalf("semantic replay rejected because of nil versus empty reasons: %v", err)
	}
}

func TestReservationTransitionRejectsProoflessReleaseStaleRenewAndTimeRegression(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	active := reservationFixture(t, now, ReservationActive)
	cas := ReservationCASV1{RequestID: "request", ReservationID: active.ReservationID, ExpectedRevision: active.ReservationRevision}
	if err := ValidateReservationTransition(active, transitionedReservation(active, ReservationReleased), cas, ReservationMutationRelease, now); err == nil {
		t.Fatal("release transition accepted without verified-detachment proof")
	}
	if err := ValidateReservationReleaseTransition(active, transitionedReservation(active, ReservationReleased), ReleaseReservationRequestV1{
		RequestID: "request", ReservationID: active.ReservationID, ExpectedRevision: active.ReservationRevision,
		VerifiedDetachedRevision: strings.Repeat("d", 64),
	}, now); err != nil {
		t.Fatalf("verified release rejected: %v", err)
	}
	expiredNext := transitionedReservation(active, ReservationActive)
	if err := ValidateReservationTransition(active, expiredNext, cas, ReservationMutationRenew, now.Add(10*time.Minute)); err == nil {
		t.Fatal("expired ACTIVE reservation renewed without reconciliation")
	}
	reserved := reservationFixture(t, now, ReservationReserved)
	reservedCAS := ReservationCASV1{RequestID: "request", ReservationID: reserved.ReservationID, ExpectedRevision: reserved.ReservationRevision}
	if err := ValidateReservationTransition(reserved, transitionedReservation(reserved, ReservationReserved), reservedCAS, ReservationMutationRenew, now); err == nil {
		t.Fatal("RESERVED reservation renewed outside the legal matrix")
	}
	active.RenewedAt += 30
	active.FreshnessExpiresAt += 30
	regressed := transitionedReservation(active, ReservationActive)
	regressed.RenewedAt = active.RenewedAt - 1
	if err := ValidateReservationTransition(active, regressed, cas, ReservationMutationRenew, now); err == nil {
		t.Fatal("reservation renewal time regressed")
	}
}

func reservationFixture(t *testing.T, now time.Time, state ReservationState) ProviderTargetReservationV1 {
	t.Helper()
	reference, err := ReferenceV2FromTarget(validTargetV2(t, "provider", "target", "endpoint", now))
	if err != nil {
		t.Fatal(err)
	}
	value := ProviderTargetReservationV1{
		Schema: ProviderTargetReservationSchemaV1, ReservationID: "reservation-1", ReservationRevision: "revision-1",
		HolderID: "operation-1", Purpose: ReservationPurposeNativeFallback, ExactTargetReference: reference, State: state,
		IssuedAt: now.Unix(), RenewedAt: now.Unix(), FreshnessExpiresAt: now.Add(5 * time.Minute).Unix(),
	}
	if state == ReservationReleased {
		value.ReleasedAt = now.Add(time.Minute).Unix()
	}
	return value
}

func transitionedReservation(current ProviderTargetReservationV1, nextState ReservationState) ProviderTargetReservationV1 {
	next := current
	next.State = nextState
	next.ReservationRevision += "-next"
	next.RenewedAt++
	next.FreshnessExpiresAt++
	if nextState == ReservationReleased {
		next.ReleasedAt = next.RenewedAt
	}
	return next
}

func validTargetV2(t *testing.T, providerID, targetID, endpointID string, now time.Time) FallbackTargetV2 {
	t.Helper()
	latency := uint32(12)
	target, err := FinalizeFallbackTargetV2(FallbackTargetV2{
		Schema:   TargetSchemaV2,
		Identity: TargetIdentity{ProviderID: providerID, TargetID: targetID},
		Publish:  PublishFactsV2{Revision: "publish-1", ContentDigest: strings.Repeat("a", 64)},
		Endpoint: EndpointV2{
			EndpointID: endpointID, Network: hostresources.NetworkTCP, AddressFamily: hostresources.AddressFamilyIPv4,
			Address: "127.0.0.1", Port: 8443, Local: true, TransportSecurity: TransportSecurityTLS,
			ApplicationProtocols: []ApplicationProtocol{ApplicationProtocolHTTP2, ApplicationProtocolHTTP11},
			AcceptedServerNames:  []string{"www.example.com", "example.com"}, ProxyProtocol: hostresources.CapabilityNo,
			CanReachManagement: hostresources.CapabilityNo,
		},
		Health:           HealthV2{Readiness: ReadinessReady, ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), ConnectFirstByteP95MS: &latency},
		Capacity:         CapacityV2{State: CapacityReady, ReservationSlotsTotal: 8, ReservationSlotsUsed: 1, ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()},
		ProviderRevision: "provider-1", Source: "provider_inventory", ConfidenceBP: 10_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	return target
}
