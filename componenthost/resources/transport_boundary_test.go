package resources

import (
	"testing"
	"time"
)

func TestTransportFactsRejectIncompleteAndContradictoryEvidence(t *testing.T) {
	now := time.Unix(1000, 0)
	for name, mutate := range map[string]func(*InboundTransportCapabilityV2){
		"configuration revision":  func(f *InboundTransportCapabilityV2) { f.ConfigurationRevision = "" },
		"runtime generation":      func(f *InboundTransportCapabilityV2) { f.RuntimeGenerationRevision = "" },
		"provider identity":       func(f *InboundTransportCapabilityV2) { f.ProviderID = "" },
		"expired observation":     func(f *InboundTransportCapabilityV2) { f.ExpiresAt = now.Unix() },
		"contradictory UDP claim": func(f *InboundTransportCapabilityV2) { f.EffectiveNetworks = []Network{NetworkTCP} },
	} {
		t.Run(name, func(t *testing.T) {
			fact := validTransportFact(now, "core:inbound:1")
			mutate(&fact)
			fact = FinalizeInboundTransportCapabilityV2(fact)
			if fact.Validate(now) == nil {
				t.Fatal("incomplete or contradictory transport evidence accepted")
			}
		})
	}
}

func TestUnknownRuntimeCapabilityCannotClaimActionableQUIC(t *testing.T) {
	now := time.Unix(1000, 0)
	fact := validTransportFact(now, "core:inbound:1")
	fact.StrategyClass = TransportQUICNative
	fact.BuildFeature.State = RuntimeFeatureUnknown
	fact.BuildFeature = FinalizeRuntimeBuildFeatureV1(fact.BuildFeature)
	fact = FinalizeInboundTransportCapabilityV2(fact)
	if fact.Validate(now) == nil {
		t.Fatal("UNKNOWN runtime capability admitted an actionable QUIC socket")
	}
	fact.ActionableDirectUDPSocket = false
	fact = FinalizeInboundTransportCapabilityV2(fact)
	if err := fact.Validate(now); err != nil {
		t.Fatalf("non-actionable UNKNOWN evidence cannot be represented: %v", err)
	}
}
