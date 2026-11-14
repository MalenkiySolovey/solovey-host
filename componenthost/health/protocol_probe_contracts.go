package health

import (
	context "context"
	errors "errors"
	hostresources "github.com/MalenkiySolovey/solovey-host/componenthost/resources"
	netip "net/netip"
	sort "sort"
	strings "strings"
	time "time"
)

const (
	ProtocolProbeCapabilitySchemaV1  = "solovey-ui/protocol-probe-capability/v1"
	ProtocolProbeObservationSchemaV1 = "solovey-ui/protocol-probe-observation/v1"
	MaxProtocolProbeDuration         = 5 * time.Second
	MaxProtocolProbeFreshness        = time.Minute
	MaxProtocolProbeProvidersV1      = 128
)

type ProtocolProbeTargetV1 struct {
	ResourceID            string
	EndpointID            string
	ProtocolClass         hostresources.InboundTransportClass
	RuntimeRevision       string
	CapabilityRevision    string
	ConfigurationRevision string
	SocketRevision        string
	AddressFamily         hostresources.AddressFamily
	ConfiguredBind        string
	Port                  uint16
}

type ProtocolProbeCapabilityV1 struct {
	Schema           string                              `json:"schema"`
	ProviderID       string                              `json:"providerId"`
	ProviderInstance string                              `json:"providerInstance"`
	ResourceID       string                              `json:"resourceId"`
	EndpointID       string                              `json:"endpointId"`
	ProtocolClass    hostresources.InboundTransportClass `json:"protocolClass"`
	Available        bool                                `json:"available"`
	Revision         string                              `json:"revision"`
	ReasonCodes      []string                            `json:"reasonCodes,omitempty"`
}

type ProtocolProbeRequestV1 struct {
	Target               ProtocolProbeTargetV1
	ContributionRevision string
	CompositionRevision  string
	ManagedPlanRevision  string
	ProviderInstance     string
	MinimumGeneration    uint64
	NotBeforeUnixNano    int64
	ChallengeRevision    string
}

type ProtocolProbeObservationV1 struct {
	Schema                string                              `json:"schema"`
	ProviderID            string                              `json:"providerId"`
	ProviderInstance      string                              `json:"providerInstance"`
	ResourceID            string                              `json:"resourceId"`
	EndpointID            string                              `json:"endpointId"`
	ProtocolClass         hostresources.InboundTransportClass `json:"protocolClass"`
	RuntimeRevision       string                              `json:"runtimeRevision"`
	CapabilityRevision    string                              `json:"capabilityRevision"`
	ConfigurationRevision string                              `json:"configurationRevision"`
	SocketRevision        string                              `json:"socketRevision"`
	ContributionRevision  string                              `json:"contributionRevision"`
	CompositionRevision   string                              `json:"compositionRevision"`
	ManagedPlanRevision   string                              `json:"managedPlanRevision"`
	ChallengeRevision     string                              `json:"challengeRevision"`
	Generation            uint64                              `json:"generation"`
	ProbeID               string                              `json:"probeId"`
	StartedUnixNano       int64                               `json:"startedUnixNano"`
	CompletedUnixNano     int64                               `json:"completedUnixNano"`
	ExpiresUnixNano       int64                               `json:"expiresUnixNano"`
	Passed                bool                                `json:"passed"`
	RequestResponse       bool                                `json:"requestResponse"`
	ExactTarget           bool                                `json:"exactTarget"`
	ResponderRevision     string                              `json:"responderRevision"`
	Revision              string                              `json:"revision"`
	ReasonCodes           []string                            `json:"reasonCodes,omitempty"`
}

type ProtocolProbeProviderV1 interface {
	ProviderID() string
	ProviderInstance() string
	Capability(context.Context, ProtocolProbeTargetV1) ProtocolProbeCapabilityV1
	Probe(context.Context, ProtocolProbeRequestV1) (ProtocolProbeObservationV1, error)
}

func validProtocolCapability(value ProtocolProbeCapabilityV1, target ProtocolProbeTargetV1) bool {
	return value.Schema == ProtocolProbeCapabilitySchemaV1 && value.ProviderID != "" && value.ProviderInstance != "" && value.ResourceID == target.ResourceID && value.EndpointID == target.EndpointID && value.ProtocolClass == target.ProtocolClass && value.Available && value.Revision == hostresources.Revision(protocolCapabilityRevisionInput(value))
}

func FinalizeProtocolProbeCapabilityV1(value ProtocolProbeCapabilityV1) ProtocolProbeCapabilityV1 {
	value.Schema = ProtocolProbeCapabilitySchemaV1
	value.ReasonCodes = sortedCodes(value.ReasonCodes)
	value.Revision = hostresources.Revision(protocolCapabilityRevisionInput(value))
	return value
}

func protocolCapabilityRevisionInput(value ProtocolProbeCapabilityV1) ProtocolProbeCapabilityV1 {
	value.Revision = ""
	return value
}

func FinalizeProtocolProbeObservationV1(value ProtocolProbeObservationV1) ProtocolProbeObservationV1 {
	value.Schema = ProtocolProbeObservationSchemaV1
	value.ReasonCodes = sortedCodes(value.ReasonCodes)
	value.Revision = hostresources.Revision(protocolObservationRevisionInput(value))
	return value
}

func protocolObservationRevisionInput(value ProtocolProbeObservationV1) ProtocolProbeObservationV1 {
	value.ChallengeRevision = ""
	value.Generation = 0
	value.ProbeID = ""
	value.StartedUnixNano = 0
	value.CompletedUnixNano = 0
	value.ExpiresUnixNano = 0
	value.ResponderRevision = ""
	value.Revision = ""
	return value
}

func validateProtocolObservation(value ProtocolProbeObservationV1, request ProtocolProbeRequestV1, capability ProtocolProbeCapabilityV1) error {
	now := time.Now().UTC().UnixNano()
	if value.Schema != ProtocolProbeObservationSchemaV1 || value.ProviderID != capability.ProviderID || value.ProviderInstance != capability.ProviderInstance ||
		value.ResourceID != request.Target.ResourceID || value.EndpointID != request.Target.EndpointID || value.ProtocolClass != request.Target.ProtocolClass ||
		value.RuntimeRevision != request.Target.RuntimeRevision || value.CapabilityRevision != request.Target.CapabilityRevision || value.ConfigurationRevision != request.Target.ConfigurationRevision || value.SocketRevision != request.Target.SocketRevision ||
		value.ContributionRevision != request.ContributionRevision || value.CompositionRevision != request.CompositionRevision || value.ManagedPlanRevision != request.ManagedPlanRevision ||
		value.ChallengeRevision != request.ChallengeRevision || value.Generation < request.MinimumGeneration || value.ProbeID == "" || value.StartedUnixNano < request.NotBeforeUnixNano ||
		value.CompletedUnixNano < value.StartedUnixNano || value.CompletedUnixNano > now || value.CompletedUnixNano-value.StartedUnixNano > MaxProtocolProbeDuration.Nanoseconds() ||
		value.ExpiresUnixNano <= now || value.ExpiresUnixNano-value.CompletedUnixNano > MaxProtocolProbeFreshness.Nanoseconds() ||
		!value.Passed || !value.RequestResponse || !value.ExactTarget || value.ResponderRevision != hostresources.Revision(struct{ Challenge, Probe string }{value.ChallengeRevision, value.ProbeID}) ||
		len(value.ReasonCodes) != 0 || value.Revision != hostresources.Revision(protocolObservationRevisionInput(value)) {
		return errors.New("protocol_probe_observation_invalid")
	}
	return nil
}

func validProtocolProbeRequest(value ProtocolProbeRequestV1) bool {
	address, err := netip.ParseAddr(strings.TrimSpace(value.Target.ConfiguredBind))
	if err != nil || value.Target.ResourceID == "" || value.Target.EndpointID == "" || value.Target.ProtocolClass == "" || value.Target.Port == 0 || value.ProviderInstance == "" || value.NotBeforeUnixNano <= 0 ||
		!validProbeRevision(value.Target.RuntimeRevision) || !validProbeRevision(value.Target.CapabilityRevision) || !validProbeRevision(value.Target.ConfigurationRevision) || !validProbeRevision(value.Target.SocketRevision) ||
		!validProbeRevision(value.ContributionRevision) || !validProbeRevision(value.CompositionRevision) || !validProbeRevision(value.ManagedPlanRevision) {
		return false
	}
	return value.Target.AddressFamily == hostresources.AddressFamilyIPv4 && address.Is4() || value.Target.AddressFamily == hostresources.AddressFamilyIPv6 && address.Is6()
}

func validProbeRevision(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}

func sortedCodes(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
