package health

import (
	context "context"
	hostresources "github.com/MalenkiySolovey/solovey-host/componenthost/resources"
	testing "testing"
	time "time"
)

type protocolProbeFixture struct {
	mutate func(ProtocolProbeRequestV1, *ProtocolProbeObservationV1)
}

func (*protocolProbeFixture) ProviderID() string { return "fixture-protocol-probe" }

func (*protocolProbeFixture) ProviderInstance() string { return "fixture-instance-1" }

func (p *protocolProbeFixture) Capability(_ context.Context, target ProtocolProbeTargetV1) ProtocolProbeCapabilityV1 {
	return FinalizeProtocolProbeCapabilityV1(ProtocolProbeCapabilityV1{ProviderID: p.ProviderID(), ProviderInstance: p.ProviderInstance(), ResourceID: target.ResourceID, EndpointID: target.EndpointID, ProtocolClass: target.ProtocolClass, Available: true})
}

func (p *protocolProbeFixture) Probe(_ context.Context, request ProtocolProbeRequestV1) (ProtocolProbeObservationV1, error) {
	started := time.Now().UTC().UnixNano()
	if started <= request.NotBeforeUnixNano {
		started = request.NotBeforeUnixNano + 1
	}
	probeID := hostresources.Revision(struct {
		Challenge  string
		Generation uint64
	}{request.ChallengeRevision, request.MinimumGeneration})
	value := ProtocolProbeObservationV1{ProviderID: p.ProviderID(), ProviderInstance: p.ProviderInstance(), ResourceID: request.Target.ResourceID, EndpointID: request.Target.EndpointID,
		ProtocolClass: request.Target.ProtocolClass, RuntimeRevision: request.Target.RuntimeRevision, CapabilityRevision: request.Target.CapabilityRevision,
		ConfigurationRevision: request.Target.ConfigurationRevision, SocketRevision: request.Target.SocketRevision, ContributionRevision: request.ContributionRevision,
		CompositionRevision: request.CompositionRevision, ManagedPlanRevision: request.ManagedPlanRevision, ChallengeRevision: request.ChallengeRevision,
		Generation: request.MinimumGeneration, ProbeID: probeID, StartedUnixNano: started, CompletedUnixNano: started,
		ExpiresUnixNano: started + int64(30*time.Second), Passed: true, RequestResponse: true, ExactTarget: true,
		ResponderRevision: hostresources.Revision(struct{ Challenge, Probe string }{request.ChallengeRevision, probeID})}
	if p.mutate != nil {
		p.mutate(request, &value)
	}
	return FinalizeProtocolProbeObservationV1(value), nil
}

func protocolProbeRequestFixture() ProtocolProbeRequestV1 {
	revision := hostresources.Revision("fixture")
	return ProtocolProbeRequestV1{Target: ProtocolProbeTargetV1{ResourceID: "core:inbound:1", EndpointID: "udp:ipv4:443", ProtocolClass: hostresources.TransportPlainUDP,
		RuntimeRevision: revision, CapabilityRevision: revision, ConfigurationRevision: revision, SocketRevision: revision,
		AddressFamily: hostresources.AddressFamilyIPv4, ConfiguredBind: "127.0.0.1", Port: 443}, ContributionRevision: revision,
		CompositionRevision: revision, ManagedPlanRevision: revision, ProviderInstance: "fixture-instance-1", NotBeforeUnixNano: time.Now().Add(-time.Second).UnixNano()}
}

func TestProtocolProbeSemanticRevisionIgnoresProofClockAndGeneration(t *testing.T) {
	request := protocolProbeRequestFixture()
	provider := &protocolProbeFixture{}
	request.ChallengeRevision = hostresources.Revision("challenge-one")
	request.MinimumGeneration = 1
	first, err := provider.Probe(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ChallengeRevision = hostresources.Revision("challenge-two")
	request.MinimumGeneration = 99
	second, err := provider.Probe(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != second.Revision {
		t.Fatalf("volatile proof identity changed semantic revision: %s != %s", first.Revision, second.Revision)
	}
	second.Passed = false
	second = FinalizeProtocolProbeObservationV1(second)
	if first.Revision == second.Revision {
		t.Fatal("semantic health result did not change revision")
	}
}

type localProxyProbeFixture struct {
	mutate func(LocalProxyProbeRequestV1, *LocalProxyProbeObservationV1)
}

func (*localProxyProbeFixture) ProviderID() string { return "core-sing-box-local-proxy-probe-v1" }

func (*localProxyProbeFixture) ProviderInstance() string { return "fixture-local-proxy-instance" }

func (p *localProxyProbeFixture) Capability(_ context.Context, target LocalProxyProbeTargetV1) LocalProxyProbeCapabilityV1 {
	return FinalizeLocalProxyProbeCapabilityV1(LocalProxyProbeCapabilityV1{
		ProviderID: p.ProviderID(), ProviderInstance: p.ProviderInstance(), ResourceID: target.ResourceID,
		EndpointID: target.EndpointID, Protocol: target.Protocol, Available: true,
	})
}

func (p *localProxyProbeFixture) Probe(_ context.Context, request LocalProxyProbeRequestV1) (LocalProxyProbeObservationV1, error) {
	started := time.Now().UTC().UnixNano()
	if started <= request.NotBeforeUnixNano {
		started = request.NotBeforeUnixNano + 1
	}
	probeID := hostresources.Revision(struct {
		Challenge  string
		Generation uint64
	}{request.ChallengeRevision, request.MinimumGeneration})
	sinkRevision := hostresources.Revision(struct{ Challenge string }{request.ChallengeRevision})
	target := request.Target
	value := LocalProxyProbeObservationV1{
		ProviderID: p.ProviderID(), ProviderInstance: p.ProviderInstance(), ResourceID: target.ResourceID,
		EndpointID: target.EndpointID, Protocol: target.Protocol, ConfigurationRevision: target.ConfigurationRevision,
		RuntimeRevision: target.RuntimeRevision, FactRevision: target.FactRevision,
		ListenerObservationRevision: target.ListenerObservationRevision, AuthenticationRevision: target.AuthenticationRevision,
		TLSRevision: target.TLSRevision, SystemProxyRevision: target.SystemProxyRevision,
		LeaseID: target.LeaseID, LeaseRevision: target.LeaseRevision, LeaseState: target.LeaseState,
		OperationID: target.OperationID, OperationRevision: target.OperationRevision,
		PlanRevision: target.PlanRevision, MarkerRevision: target.MarkerRevision,
		ChallengeRevision: request.ChallengeRevision, Generation: request.MinimumGeneration, ProbeID: probeID,
		StartedUnixNano: started, CompletedUnixNano: started, ExpiresUnixNano: started + int64(30*time.Second),
		Passed: true, PositiveTransaction: true, MissingAuthenticationDenied: true,
		InvalidAuthenticationDenied: true, ExactTarget: true, ExactSink: true, SinkRevision: sinkRevision,
		ResponderRevision: hostresources.Revision(struct{ Challenge, Probe, Sink string }{request.ChallengeRevision, probeID, sinkRevision}),
	}
	if p.mutate != nil {
		p.mutate(request, &value)
	}
	return FinalizeLocalProxyProbeObservationV1(value), nil
}

func localProxyProbeRequestFixture(protocol hostresources.LocalProxyProtocolV1) LocalProxyProbeRequestV1 {
	revision := hostresources.Revision("fixture")
	return LocalProxyProbeRequestV1{
		Target: LocalProxyProbeTargetV1{
			ProviderID: "core", ResourceID: "core:inbound:17", EndpointID: "tcp:ipv4:1080", Protocol: protocol,
			ConfigurationRevision: revision, RuntimeRevision: revision, FactRevision: revision,
			ListenerObservationRevision: revision, AuthenticationRevision: revision, TLSRevision: revision,
			SystemProxyRevision: revision, LeaseID: "lease-1", LeaseRevision: revision,
			LeaseState: hostresources.EndpointLeaseMutationPending, OperationID: "operation-1", OperationRevision: 2,
			PlanRevision: revision, MarkerRevision: revision,
		},
		ProviderInstance: "fixture-local-proxy-instance", NotBeforeUnixNano: time.Now().Add(-time.Second).UnixNano(),
	}
}

type interceptionProbeFixture struct {
	id     string
	mutate func(*InterceptionProbeObservationV1)
}

func (p interceptionProbeFixture) ProviderID() string { return p.id }

func (interceptionProbeFixture) SupportsInterceptionProbeV1(InterceptionProbeTargetV1) bool {
	return true
}

func (p interceptionProbeFixture) ProbeInterceptionV1(_ context.Context, request InterceptionProbeRequestV1) (InterceptionProbeObservationV1, error) {
	value := InterceptionProbeObservationV1{
		Schema: InterceptionProbeObservationSchemaV1, ProviderID: p.id,
		TargetRevision: request.Target.Revision(), OperationID: request.OperationID,
		MarkerRevision: request.MarkerRevision, RuntimeGeneration: request.RuntimeGeneration,
		Challenge: request.Challenge, StartedAt: request.MarkerAt + 1, CompletedAt: request.MarkerAt + 2,
		RequestSent: true, ResponseReceived: true, IntendedInboundObserved: true,
		IntendedSinkObserved: true, WrongSinkUntouched: true,
		OriginalDestinationPreserved: true, SourcePreserved: true,
		ManagementPreserved: true, BoundedFlowExpired: request.Target.Network == "udp",
	}
	if p.mutate != nil {
		p.mutate(&value)
	}
	return value, nil
}

func interceptionProbeRequestFixture() InterceptionProbeRequestV1 {
	digest := hostresources.Revision("fixture")
	return InterceptionProbeRequestV1{
		Target: InterceptionProbeTargetV1{
			Schema: InterceptionProbeTargetSchemaV1, ProviderID: "core", ResourceID: "inbound:1",
			EndpointID: "endpoint:1", Kind: "TPROXY", Network: "udp", AddressFamily: "ipv4",
			IngressScopeProviderID: "network-owner", IngressScopeID: "scope:eth0",
			IngressScopeRevision: digest, InterceptionFactRevision: digest, RuntimeRevision: digest,
			ListenerRevision: digest, ManagedCandidateRevision: digest, PolicyRoutingRevision: digest,
			ManagementExclusionRevision: digest, HealthTransactionRevision: digest,
		},
		OperationID: "operation-1", MarkerRevision: digest, RuntimeGeneration: digest,
		Challenge: "challenge-1", MarkerAt: time.Now().UTC().Unix() - 3, DeadlineAt: time.Now().UTC().Unix() + 10,
	}
}
