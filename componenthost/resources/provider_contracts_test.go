package resources

import (
	json "encoding/json"
	netip "net/netip"
	strings "strings"
	testing "testing"
	time "time"
)

func (f FrontingBackendFactV1) ExpiresAtTime() time.Time { return time.Unix(f.ExpiresAt, 0) }

func TestEndpointLeaseProviderRequestsAreBoundedAndExact(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	reference, err := ReferenceFrontingBackendV1(frontingFactFixture(t, now, "127.0.0.1", AddressFamilyIPv4), ProxyModeOff, now)
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	acquire := AcquireEndpointLeaseRequestV1{RequestID: "request", HolderID: "holder", Purpose: EndpointLeasePurposeL4FrontingV1,
		ExactReference: reference, FreshnessSeconds: 60}
	if err := acquire.Validate(); err != nil {
		t.Fatal(err)
	}
	acquire.Purpose = "HTTP_FRONTING"
	if acquire.Validate() == nil {
		t.Fatal("unknown lease purpose was accepted")
	}
	if err := (MutateEndpointLeaseRequestV1{RequestID: "request", LeaseID: "lease", ExpectedRevision: digest, FreshnessSeconds: 60}).Validate(true); err != nil {
		t.Fatal(err)
	}
	if err := (MutateEndpointLeaseRequestV1{RequestID: "request", LeaseID: "lease", ExpectedRevision: digest}).Validate(false); err != nil {
		t.Fatal(err)
	}
	if err := (ReleaseEndpointLeaseRequestV1{RequestID: "request", LeaseID: "lease", ExpectedRevision: digest, DetachmentRevision: digest}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (GetEndpointLeaseRequestV1{LeaseID: "lease"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (ListEndpointLeasesRequestV1{HolderID: "holder", Limit: MaxEndpointLeasePageV1}).Validate(); err != nil {
		t.Fatal(err)
	}
	if (ListEndpointLeasesRequestV1{HolderID: "holder", Limit: MaxEndpointLeasePageV1 + 1}).Validate() == nil {
		t.Fatal("oversized lease page was accepted")
	}
}

func TestEndpointLeasePureCASAndTransitionRules(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	reference, _ := ReferenceFrontingBackendV1(frontingFactFixture(t, now, "127.0.0.1", AddressFamilyIPv4), ProxyModeOff, now)
	current, err := FinalizeEndpointLeaseV1(EndpointLeaseV1{LeaseID: "lease", AuthorityProviderID: reference.ProviderID, HolderID: "holder",
		ExactReference: reference, State: EndpointLeaseReserved, IssuedAt: now.Unix(), RenewedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	next := current
	next.State, next.RenewedAt, next.ExpiresAt = EndpointLeaseMutationPending, now.Add(time.Second).Unix(), now.Add(2*time.Minute).Unix()
	next, err = FinalizeEndpointLeaseV1(next)
	if err != nil {
		t.Fatal(err)
	}
	cas := EndpointLeaseCASV1{RequestID: "request", LeaseID: current.LeaseID, ExpectedRevision: current.LeaseRevision}
	if err := ValidateEndpointLeaseTransitionV1(current, next, cas, EndpointLeaseFence, now); err != nil {
		t.Fatal(err)
	}
	cas.ExpectedRevision = strings.Repeat("f", 64)
	if ValidateEndpointLeaseTransitionV1(current, next, cas, EndpointLeaseFence, now) == nil {
		t.Fatal("stale lease CAS was accepted")
	}
}

func TestFrontingBackendEndpointRequiresExactProviderFact(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	fact := frontingFactFixture(t, now, "127.0.0.1", AddressFamilyIPv4)
	reference, err := ReferenceFrontingBackendV1(fact, ProxyModeOff, now)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := ResolveFrontingBackendEndpointV1(reference, fact, now)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Address != netip.MustParseAddr("127.0.0.1") || endpoint.Port != 9443 || endpoint.Network != NetworkTCP || endpoint.AddressFamily != AddressFamilyIPv4 {
		t.Fatalf("unexpected execution endpoint: %#v", endpoint)
	}
	stale := fact
	stale.ProviderRevision = "provider-v2"
	if _, err := ResolveFrontingBackendEndpointV1(reference, stale, now); err == nil {
		t.Fatal("stale provider fact yielded an execution endpoint")
	}
}

func TestFrontingBackendReferenceExactFreshAndRedacted(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	for _, test := range []struct {
		name, address string
		family        AddressFamily
	}{{"ipv4", "127.0.0.1", AddressFamilyIPv4}, {"ipv6", "::1", AddressFamilyIPv6}} {
		t.Run(test.name, func(t *testing.T) {
			fact := frontingFactFixture(t, now, test.address, test.family)
			first, err := ReferenceFrontingBackendV1(fact, ProxyModeOn, now)
			if err != nil {
				t.Fatal(err)
			}
			second, _ := ReferenceFrontingBackendV1(fact, ProxyModeOn, now)
			if first != second || first.CanonicalReferenceRevision == "" || ResolveExactFrontingBackendV1(first, fact, now) != nil {
				t.Fatalf("reference mismatch: %#v %#v", first, second)
			}
			encoded, err := json.Marshal(first)
			if err != nil {
				t.Fatal(err)
			}
			var decoded any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{test.address, "hostname", "proxy_pass", "path"} {
				if strings.Contains(string(encoded), forbidden) {
					t.Fatalf("reference leaked %q: %s", forbidden, encoded)
				}
			}
			if jsonContainsNumber(decoded, float64(fact.port)) {
				t.Fatalf("reference leaked backend port %d: %s", fact.port, encoded)
			}
		})
	}
	fact := frontingFactFixture(t, now, "127.0.0.1", AddressFamilyIPv4)
	payload, _ := json.Marshal(fact)
	var reconstructed FrontingBackendFactV1
	if err := json.Unmarshal(payload, &reconstructed); err != nil {
		t.Fatal(err)
	}
	if _, err := ReferenceFrontingBackendV1(reconstructed, ProxyModeOff, now); err == nil {
		t.Fatal("outward fact was reconstructed into a destination-bearing reference")
	}
}

func TestFrontingBackendReferenceRejectsStalePublicManagementAndProxyMismatch(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	fact := frontingFactFixture(t, now, "127.0.0.1", AddressFamilyIPv4)
	reference, _ := ReferenceFrontingBackendV1(fact, ProxyModeOff, now)
	stale := fact
	stale.ProviderRevision = "provider-v2"
	if ResolveExactFrontingBackendV1(reference, stale, now) == nil || ResolveExactFrontingBackendV1(reference, fact, fact.ExpiresAtTime()) == nil {
		t.Fatal("stale exact reference was accepted")
	}
	unsafeFact := fact
	unsafeFact.CanReachManagement = CapabilityUnknown
	if _, err := ReferenceFrontingBackendV1(unsafeFact, ProxyModeOff, now); err == nil {
		t.Fatal("unknown management reachability was accepted")
	}
	unsafeFact = fact
	unsafeFact.AcceptsProxyProtocol = CapabilityNo
	if _, err := ReferenceFrontingBackendV1(unsafeFact, ProxyModeOn, now); err == nil {
		t.Fatal("PROXY mismatch was accepted")
	}
	unsafeFact = fact
	unsafeFact.Classification = EndpointClassificationPublic
	if _, err := ReferenceFrontingBackendV1(unsafeFact, ProxyModeOff, now); err == nil {
		t.Fatal("public backend was accepted")
	}
	unsafeFact = fact
	unsafeFact.Ownership = FrontingBackendExternalManaged
	if _, err := ReferenceFrontingBackendV1(unsafeFact, ProxyModeOff, now); err == nil {
		t.Fatal("external-managed backend was accepted")
	}
	for _, kind := range []string{"panel_web", "subscription", "ssh", "node_control", "external_upstream"} {
		if got := frontingFactFixtureForKindNoFail(now, "127.0.0.1", AddressFamilyIPv4, kind); got == nil {
			t.Fatalf("excluded resource kind %q was accepted", kind)
		}
	}
}

func TestFrontingBackendReferenceRevisionIgnoresOnlyObservationTime(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	firstFact := frontingFactFixture(t, now, "127.0.0.1", AddressFamilyIPv4)
	refreshedFact := frontingFactFixture(t, now.Add(time.Second), "127.0.0.1", AddressFamilyIPv4)
	first, err := ReferenceFrontingBackendV1(firstFact, ProxyModeOff, now)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := ReferenceFrontingBackendV1(refreshedFact, ProxyModeOff, now.Add(time.Second))
	if err != nil || first != refreshed {
		t.Fatalf("observation refresh changed exact reference: first=%#v refreshed=%#v err=%v", first, refreshed, err)
	}
	changedFact := refreshedFact
	changedFact.HealthRevision = Revision("changed-health")
	changed, err := ReferenceFrontingBackendV1(changedFact, ProxyModeOff, now.Add(time.Second))
	if err != nil || changed.CanonicalReferenceRevision == first.CanonicalReferenceRevision {
		t.Fatalf("health change retained exact reference: changed=%#v err=%v", changed, err)
	}
}

func TestInboundTransportFactContainsNoExtensibleOrSecretFields(t *testing.T) {
	now := time.Unix(1000, 0)
	fact := validTransportFact(now, "core:inbound:1")
	if err := fact.Validate(now); err != nil {
		t.Fatal(err)
	}
	if fact.AuthenticationCount != 0 || fact.InboundTag != "" {
		t.Fatalf("unexpected identity data %#v", fact)
	}
}

func TestInterceptionContractsRejectProtocolAndScopeConfusion(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	redirectUDP := interceptionFactFixture(t, now)
	redirectUDP.Network = NetworkUDP
	if _, err := FinalizeInterceptionFactV1(redirectUDP); err == nil {
		t.Fatal("Redirect UDP was accepted")
	}
	tproxy := interceptionFactFixture(t, now)
	tproxy.Kind = InterceptionTProxyV1
	tproxy.TransparentSocketRequired = true
	tproxy.PolicyRoutingRequired = false
	if _, err := FinalizeInterceptionFactV1(tproxy); err == nil {
		t.Fatal("TProxy without policy routing was accepted")
	}
	if _, err := FinalizeIngressScopeFactV1(ForwardedIngressScopeFactV1{
		ProviderID: "host-network", ProviderRevision: IngressScopeProviderRevisionV1,
		ScopeID: "scope:1", InterfaceName: "lo", InterfaceIndex: 1,
		InterfaceRevision: Revision("lo"), AddressFamily: AddressFamilyIPv4,
		Ownership: IngressScopeProviderManagedV1, ForwardedIngress: true, Loopback: true,
		ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(),
	}); err == nil {
		t.Fatal("loopback was accepted as forwarded ingress")
	}
}

func TestInterceptionFactRevisionIgnoresObservationClockButExpiryFailsClosed(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	first := interceptionFactFixture(t, now)
	second := first
	second.ObservedAt = now.Add(10 * time.Second).Unix()
	second.ExpiresAt = now.Add(70 * time.Second).Unix()
	second, err := FinalizeInterceptionFactV1(second)
	if err != nil {
		t.Fatalf("refinalize fixture: %v", err)
	}
	if first.FactRevision != second.FactRevision {
		t.Fatal("volatile observation time changed the semantic fact revision")
	}
	if first.Validate(now.Add(61*time.Second)) == nil {
		t.Fatal("expired fact remained valid")
	}
}

func TestLocalProxyEligibilityMatrixAndExactReferenceBoundary(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	for name, fact := range map[string]LocalProxyFactV1{
		"loopback no auth": localProxyFactFixture("socks", LocalProxyExposureLoopback, LocalProxyAuthenticationAbsent),
		"private auth":     localProxyFactFixture("http", LocalProxyExposurePrivate, LocalProxyAuthenticationPresent),
		"mixed auth":       localProxyFactFixture("mixed", LocalProxyExposurePrivate, LocalProxyAuthenticationPresent),
	} {
		t.Run(name, func(t *testing.T) {
			if err := fact.Actionable(now); err != nil {
				t.Fatalf("actionable fact rejected: %v", err)
			}
			reference, err := ReferenceLocalProxyV1(fact, now)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(reference)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"configuredBind", "configuredPort", "observedBind", "observedPort", "127.0.0.1", "10.0.0.8", "username", "password", "destination"} {
				if strings.Contains(string(payload), forbidden) {
					t.Fatalf("reference leaked forbidden authority %q: %s", forbidden, payload)
				}
			}
		})
	}
	for name, fact := range map[string]LocalProxyFactV1{
		"private no auth": localProxyFactFixture("socks", LocalProxyExposurePrivate, LocalProxyAuthenticationAbsent),
		"public":          localProxyFactFixture("http", LocalProxyExposurePublic, LocalProxyAuthenticationPresent),
		"wildcard":        localProxyFactFixture("mixed", LocalProxyExposureWildcard, LocalProxyAuthenticationPresent),
		"unknown auth":    localProxyFactFixture("socks", LocalProxyExposureLoopback, LocalProxyAuthenticationUnknown),
	} {
		t.Run(name, func(t *testing.T) {
			if err := fact.Actionable(now); err == nil {
				t.Fatal("unsafe fact became actionable")
			}
		})
	}
}

func TestLocalProxyFactRejectsIncompleteProtocolAndUDPOrSystemProxyClaims(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	tests := []struct {
		name       string
		fact       LocalProxyFactV1
		diagnostic bool
	}{
		{"socks without socks5", localProxyFactFixture("socks", LocalProxyExposureLoopback, LocalProxyAuthenticationAbsent), false},
		{"http without connect", localProxyFactFixture("http", LocalProxyExposureLoopback, LocalProxyAuthenticationAbsent), false},
		{"wrong dependent udp", localProxyFactFixture("mixed", LocalProxyExposureLoopback, LocalProxyAuthenticationAbsent), false},
		{"system proxy enabled", localProxyFactFixture("http", LocalProxyExposureLoopback, LocalProxyAuthenticationAbsent), true},
	}
	tests[0].fact.Protocols = []LocalProxyProtocolV1{LocalProxyProtocolSOCKS4}
	tests[1].fact.Protocols = []LocalProxyProtocolV1{LocalProxyProtocolHTTPForward}
	tests[2].fact.DependentUDPAssociation = false
	tests[3].fact.SystemProxy = LocalProxySystemProxyEnabled
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.fact.FactRevision = Revision(localProxyFactRevisionInput(test.fact))
			if test.diagnostic {
				if err := test.fact.Validate(now); err != nil {
					t.Fatalf("diagnostic fact must remain valid: %v", err)
				}
				if _, err := ReferenceLocalProxyV1(test.fact, now); err == nil {
					t.Fatal("enabled system proxy produced an actionable reference")
				}
				return
			}
			if err := test.fact.Validate(now); err == nil {
				t.Fatal("invalid protocol or UDP semantics were accepted")
			}
		})
	}
}

func TestLocalProxyFactSeparatesFreshnessFromSemanticRevisionAndFailsClosedOnDrift(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	first := localProxyFactFixture("mixed", LocalProxyExposureLoopback, LocalProxyAuthenticationAbsent)
	refreshed := first
	refreshed.ObservedAt, refreshed.ExpiresAt = first.ObservedAt+10, first.ExpiresAt+10
	if err := refreshed.Validate(now.Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if first.FactRevision != refreshed.FactRevision {
		t.Fatal("freshness clock changed semantic fact revision")
	}
	drifted := first
	drifted.TLS = LocalProxyTLSEnabled
	if err := drifted.Validate(now); err == nil {
		t.Fatal("TLS drift with stale fact revision was accepted")
	}
	offline := first
	offline.EffectiveRuntimeRevision = ""
	offline.RuntimeReady = false
	offline.HealthCapabilityReady = false
	offline.ReasonCodes = []string{"RUNTIME_CONFIGURATION_UNPROVEN"}
	offline.FactRevision = Revision(localProxyFactRevisionInput(offline))
	if err := offline.Validate(now); err != nil {
		t.Fatalf("diagnostic offline fact rejected: %v", err)
	}
	if err := offline.Actionable(now); err == nil {
		t.Fatal("offline fact became actionable")
	}
}

func TestLocalProxyLeaseTransitionsRequireCASAndPreserveExactReference(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	reference, err := ReferenceLocalProxyV1(localProxyFactFixture("socks", LocalProxyExposureLoopback, LocalProxyAuthenticationAbsent), now)
	if err != nil {
		t.Fatal(err)
	}
	current, err := FinalizeLocalProxyGuardLeaseV1(LocalProxyGuardLeaseV1{
		LeaseID: "local-proxy-lease-1", AuthorityProviderID: "core", HolderID: "operation-1",
		ExactReference: reference, State: EndpointLeaseReserved, IssuedAt: now.Unix(),
		RenewedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), ReasonCodes: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	next := current
	next.State = EndpointLeaseMutationPending
	next, err = FinalizeLocalProxyGuardLeaseV1(next)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalProxyGuardLeaseTransitionV1(current, next, current.LeaseRevision, EndpointLeaseFence, now); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalProxyGuardLeaseTransitionV1(current, next, Revision("stale"), EndpointLeaseFence, now); err == nil {
		t.Fatal("stale CAS revision was accepted")
	}
	next.ExactReference.Protocols = []LocalProxyProtocolV1{LocalProxyProtocolSOCKS4}
	next, _ = FinalizeLocalProxyGuardLeaseV1(next)
	if err := ValidateLocalProxyGuardLeaseTransitionV1(current, next, current.LeaseRevision, EndpointLeaseFence, now); err == nil {
		t.Fatal("lease authority reference changed during transition")
	}

	expiredAt := now.Add(2 * time.Minute)
	expired := current
	released := expired
	released.State, released.ReleasedAt = EndpointLeaseReleased, expiredAt.Unix()
	released, err = FinalizeLocalProxyGuardLeaseV1(released)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalProxyGuardLeaseTransitionV1(expired, released, expired.LeaseRevision, EndpointLeaseRelease, expiredAt); err != nil {
		t.Fatalf("exact release of expired authority must remain recoverable: %v", err)
	}
	expiredFence := expired
	expiredFence.State = EndpointLeaseMutationPending
	expiredFence, err = FinalizeLocalProxyGuardLeaseV1(expiredFence)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalProxyGuardLeaseTransitionV1(expired, expiredFence, expired.LeaseRevision, EndpointLeaseFence, expiredAt); err == nil {
		t.Fatal("expired authority crossed the mutation boundary")
	}
}

func frontingFactFixture(t *testing.T, now time.Time, address string, family AddressFamily) FrontingBackendFactV1 {
	t.Helper()
	return frontingFactFixtureForKind(t, now, address, family, string(FrontingBackendInboundResource))
}

func frontingFactFixtureForKind(t *testing.T, now time.Time, address string, family AddressFamily, kind string) FrontingBackendFactV1 {
	t.Helper()
	endpoint := PublicEndpoint{Schema: EndpointSchemaV1, ID: "endpoint",
		Key:    PublicEndpointKey{Network: NetworkTCP, AddressFamily: family, BindAddress: address, Port: 9443},
		Intent: EndpointIntentLocal, Protocol: "tcp", ProxyProtocol: CapabilityYes,
		ResourceID: "core:inbound:1", Owner: "core", OwnerRevision: "owner-v1",
		ConfigurationRevision: strings.Repeat("d", 64), ObservedAt: now.Unix(), Source: "fixture", ConfidenceBP: 10_000}
	resource := ProtectableResource{ID: endpoint.ResourceID, Kind: kind, Owner: endpoint.Owner,
		Capabilities: ProtectableResourceCapabilities{Known: true, OwnerRevision: endpoint.OwnerRevision}, Endpoints: []PublicEndpoint{endpoint}}
	fact, err := NewFrontingBackendFactV1(FrontingBackendFactV1{
		ProviderID: "provider", ContributorID: "contributor", ProviderRevision: "provider-v1",
		HealthRevision: strings.Repeat("b", 64), CapacityRevision: strings.Repeat("c", 64),
		Ownership: FrontingBackendProviderManaged, CanReachManagement: CapabilityNo, HealthReady: true, CapacityReady: true,
		ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(),
	}, resource, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	return fact
}

func frontingFactFixtureForKindNoFail(now time.Time, address string, family AddressFamily, kind string) error {
	endpoint := PublicEndpoint{Schema: EndpointSchemaV1, ID: "endpoint",
		Key:    PublicEndpointKey{Network: NetworkTCP, AddressFamily: family, BindAddress: address, Port: 9443},
		Intent: EndpointIntentLocal, Protocol: "tcp", ProxyProtocol: CapabilityNo,
		ResourceID: "core:resource:1", Owner: "core", OwnerRevision: "owner-v1",
		ConfigurationRevision: strings.Repeat("d", 64), ObservedAt: now.Unix(), Source: "fixture", ConfidenceBP: 10_000}
	resource := ProtectableResource{ID: endpoint.ResourceID, Kind: kind, Owner: endpoint.Owner,
		Capabilities: ProtectableResourceCapabilities{Known: true, OwnerRevision: endpoint.OwnerRevision}, Endpoints: []PublicEndpoint{endpoint}}
	_, err := NewFrontingBackendFactV1(FrontingBackendFactV1{ProviderID: "provider", ContributorID: "contributor",
		ProviderRevision: "provider-v1", HealthRevision: strings.Repeat("b", 64), CapacityRevision: strings.Repeat("c", 64),
		Ownership: FrontingBackendProviderManaged, CanReachManagement: CapabilityNo, HealthReady: true, CapacityReady: true,
		ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()}, resource, endpoint)
	return err
}

func interceptionFactFixture(t *testing.T, now time.Time) InterceptionInboundFactV1 {
	t.Helper()
	digest := Revision("fixture")
	fact, err := FinalizeInterceptionFactV1(InterceptionInboundFactV1{
		ProviderID: "core", ProviderRevision: InterceptionProviderRevisionV1,
		ResourceID: "inbound:7", EndpointID: "endpoint:0123456789abcdef",
		InboundDatabaseID: 7, InboundTag: "redirect-in", Kind: InterceptionRedirectV1,
		Network: NetworkTCP, AddressFamily: AddressFamilyIPv4,
		ConfiguredBind: "0.0.0.0", ConfiguredPort: 15001,
		Ownership: InterceptionProviderManagedV1, ListenerState: InterceptionListenerObservedExactV1,
		ConfigurationRevision: digest, RuntimeRevision: digest, RuntimeGenerationRevision: digest,
		ListenerRevision: digest, CoreSemanticRevision: digest, LinuxOnly: true,
		OriginalDestinationMechanism: "SO_ORIGINAL_DST", OriginalDestinationPreserved: true,
		SourcePreserved: true, RuntimeReady: true,
		ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("finalize fixture: %v", err)
	}
	return fact
}

func jsonContainsNumber(value any, wanted float64) bool {
	switch typed := value.(type) {
	case float64:
		return typed == wanted
	case []any:
		for _, item := range typed {
			if jsonContainsNumber(item, wanted) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if jsonContainsNumber(item, wanted) {
				return true
			}
		}
	}
	return false
}

func localProxyFactFixture(inboundType string, exposure LocalProxyExposureV1, authentication LocalProxyAuthenticationV1) LocalProxyFactV1 {
	now := time.Unix(1_800_000_000, 0).UTC()
	bind := "127.0.0.1"
	family := AddressFamilyIPv4
	if exposure == LocalProxyExposurePrivate {
		bind = "10.0.0.8"
	}
	if exposure == LocalProxyExposurePublic {
		bind = "203.0.113.8"
	}
	if exposure == LocalProxyExposureWildcard {
		bind = "0.0.0.0"
	}
	protocols := []LocalProxyProtocolV1{LocalProxyProtocolSOCKS5}
	switch inboundType {
	case "http":
		protocols = []LocalProxyProtocolV1{LocalProxyProtocolHTTPConnect, LocalProxyProtocolHTTPForward}
	case "mixed":
		protocols = []LocalProxyProtocolV1{LocalProxyProtocolHTTPConnect, LocalProxyProtocolHTTPForward, LocalProxyProtocolSOCKS5}
	}
	count := 0
	if authentication == LocalProxyAuthenticationPresent {
		count = 1
	}
	digest := Revision("fixture")
	fact := LocalProxyFactV1{
		Schema: LocalProxyFactSchemaV1, ProviderID: "core", ContributorID: "core",
		ResourceID: "core:inbound:17", EndpointID: "tcp:ipv4:1080", InboundDatabaseID: 17,
		InboundType: inboundType, ConfigurationRevision: digest, EffectiveRuntimeRevision: Revision("runtime"),
		RuntimeIdentityRevision: "runtime-owner-v1", ProviderRevision: LocalProxyProviderRevisionV1,
		CapabilityRevision: LocalProxyCapabilityRevisionV1, ListenerObservationRevision: Revision("listener"),
		OwnerRevision: "owner-v1", HealthRevision: Revision("health"), CapacityRevision: Revision("capacity"),
		ManagementExclusionRevision: Revision("management"), RecoveryPathRevision: Revision("recovery"),
		ConfiguredBind: bind, ConfiguredPort: 1080, AddressFamily: family,
		ObservedBind: bind, ObservedPort: 1080, ObservedAddressFamily: family,
		Exposure: exposure, Ownership: LocalProxyProviderManaged, ListenerState: LocalProxyListenerObservedExact,
		Protocols: protocols, Authentication: authentication, AuthenticationCount: count,
		AuthenticationRevision: Revision("authentication"), TLS: LocalProxyTLSDisabled, TLSRevision: Revision("tls"),
		SystemProxy: LocalProxySystemProxyDisabled, SystemProxyRevision: Revision("system-proxy"),
		DependentUDPAssociation: inboundType == "socks" || inboundType == "mixed",
		RuntimeReady:            true, HealthCapabilityReady: true, CapacityReady: true,
		ManagementCollision: CapabilityNo, RecoveryPathCollision: CapabilityNo,
		ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), ReasonCodes: []string{},
	}
	fact.FactRevision = Revision(localProxyFactRevisionInput(fact))
	return fact
}

func validTransportFact(now time.Time, resource string) InboundTransportCapabilityV2 {
	digest := Revision(resource)
	feature := FinalizeRuntimeBuildFeatureV1(RuntimeBuildFeatureV1{Feature: "with_quic", State: RuntimeFeatureSupported, RuntimeIdentity: digest, SourceRevision: digest, ModuleRevision: digest, BuildProfileRevision: digest, ObservationMethod: "compile_tag", ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
	return FinalizeInboundTransportCapabilityV2(InboundTransportCapabilityV2{ProviderID: "provider", ContributorID: "contributor", ResourceID: resource, InboundDatabaseID: 1, InboundType: "direct", StrategyClass: TransportPlainUDP, ConfigurationRevision: digest, EffectiveRuntimeRevision: digest, PinnedRuntimeIdentity: digest, BuildFeature: feature, EffectiveNetworks: []Network{NetworkUDP}, EffectiveNetworksRevision: digest, TransportRevision: digest, SocketIntentRevision: digest, AuthenticationRevision: digest, TLSSemanticRevision: digest, UDPTimeoutRevision: digest, ListenerOwnerRevision: digest, RuntimeGenerationRevision: digest, ActionableDirectUDPSocket: true, ObservedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix()})
}
