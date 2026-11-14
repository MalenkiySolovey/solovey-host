package health

import (
	"testing"

	hostresources "github.com/MalenkiySolovey/solovey-host/componenthost/resources"
)

func TestProtocolProbeValuesRejectForeignAndPreMutationEvidence(t *testing.T) {
	request := protocolProbeRequestFixture()
	request.ChallengeRevision, request.MinimumGeneration = hostresources.Revision("challenge"), 1
	provider := &protocolProbeFixture{}
	capability := provider.Capability(t.Context(), request.Target)
	request.Target.CapabilityRevision = capability.Revision
	if !ProtocolProbeRequestValidV1(request) || !ProtocolProbeCapabilityValidV1(capability, request.Target) {
		t.Fatal("valid typed probe request rejected")
	}
	observation, err := provider.Probe(t.Context(), request)
	if err != nil || ValidateProtocolProbeObservationV1(observation, request, capability) != nil {
		t.Fatal("valid exact probe observation rejected", err)
	}
	for name, mutate := range map[string]func(*ProtocolProbeObservationV1){
		"foreign resource": func(o *ProtocolProbeObservationV1) { o.ResourceID = "foreign:resource" },
		"foreign instance": func(o *ProtocolProbeObservationV1) { o.ProviderInstance = "foreign-instance" },
		"before mutation":  func(o *ProtocolProbeObservationV1) { o.StartedUnixNano = request.NotBeforeUnixNano - 1 },
		"old generation":   func(o *ProtocolProbeObservationV1) { o.Generation = 0 },
		"wrong challenge":  func(o *ProtocolProbeObservationV1) { o.ChallengeRevision = hostresources.Revision("other") },
		"send only":        func(o *ProtocolProbeObservationV1) { o.RequestResponse = false },
	} {
		t.Run(name, func(t *testing.T) {
			value := observation
			mutate(&value)
			value = FinalizeProtocolProbeObservationV1(value)
			if ValidateProtocolProbeObservationV1(value, request, capability) == nil {
				t.Fatal("foreign or stale probe evidence accepted")
			}
		})
	}
}

func TestLocalProxyProbeValuesRequireExactCurrentLeaseAndSink(t *testing.T) {
	request := localProxyProbeRequestFixture(hostresources.LocalProxyProtocolSOCKS5)
	request.ChallengeRevision, request.MinimumGeneration = hostresources.Revision("challenge"), 1
	provider := &localProxyProbeFixture{}
	capability := provider.Capability(t.Context(), request.Target)
	if !LocalProxyProbeRequestValidV1(request) || !LocalProxyProbeCapabilityValidV1(capability, request.Target) {
		t.Fatal("valid local proxy probe request rejected")
	}
	observation, err := provider.Probe(t.Context(), request)
	if err != nil || ValidateLocalProxyProbeObservationV1(observation, request, capability) != nil {
		t.Fatal("valid local proxy proof rejected", err)
	}
	for name, mutate := range map[string]func(*LocalProxyProbeObservationV1){
		"foreign lease":        func(o *LocalProxyProbeObservationV1) { o.LeaseID = "foreign-lease" },
		"stale lease revision": func(o *LocalProxyProbeObservationV1) { o.LeaseRevision = hostresources.Revision("other") },
		"old operation":        func(o *LocalProxyProbeObservationV1) { o.OperationRevision++ },
		"wrong sink":           func(o *LocalProxyProbeObservationV1) { o.ExactSink = false },
		"old generation":       func(o *LocalProxyProbeObservationV1) { o.Generation = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			value := observation
			mutate(&value)
			value = FinalizeLocalProxyProbeObservationV1(value)
			if ValidateLocalProxyProbeObservationV1(value, request, capability) == nil {
				t.Fatal("unbound proxy proof accepted")
			}
		})
	}
}
