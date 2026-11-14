package health

import (
	context "context"
	errors "errors"
	hostresources "github.com/MalenkiySolovey/solovey-host/componenthost/resources"
)

const (
	InterceptionProbeTargetSchemaV1      = "solovey-ui/interception-probe-target/v1"
	InterceptionProbeObservationSchemaV1 = "solovey-ui/interception-probe-observation/v1"
	MaxInterceptionProbeProvidersV1      = 32
)

type InterceptionProbeTargetV1 struct {
	Schema                      string `json:"schema"`
	ProviderID                  string `json:"providerId"`
	ResourceID                  string `json:"resourceId"`
	EndpointID                  string `json:"endpointId"`
	Kind                        string `json:"kind"`
	Network                     string `json:"network"`
	AddressFamily               string `json:"addressFamily"`
	IngressScopeProviderID      string `json:"ingressScopeProviderId"`
	IngressScopeID              string `json:"ingressScopeId"`
	IngressScopeRevision        string `json:"ingressScopeRevision"`
	InterceptionFactRevision    string `json:"interceptionFactRevision"`
	RuntimeRevision             string `json:"runtimeRevision"`
	ListenerRevision            string `json:"listenerRevision"`
	ManagedCandidateRevision    string `json:"managedCandidateRevision"`
	PolicyRoutingRevision       string `json:"policyRoutingRevision,omitempty"`
	ManagementExclusionRevision string `json:"managementExclusionRevision"`
	HealthTransactionRevision   string `json:"healthTransactionRevision"`
}

type InterceptionProbeRequestV1 struct {
	Target            InterceptionProbeTargetV1 `json:"target"`
	OperationID       string                    `json:"operationId"`
	MarkerRevision    string                    `json:"markerRevision"`
	RuntimeGeneration string                    `json:"runtimeGeneration"`
	Challenge         string                    `json:"challenge"`
	MarkerAt          int64                     `json:"markerAt"`
	DeadlineAt        int64                     `json:"deadlineAt"`
}

type InterceptionProbeObservationV1 struct {
	Schema                       string `json:"schema"`
	ProviderID                   string `json:"providerId"`
	TargetRevision               string `json:"targetRevision"`
	OperationID                  string `json:"operationId"`
	MarkerRevision               string `json:"markerRevision"`
	RuntimeGeneration            string `json:"runtimeGeneration"`
	Challenge                    string `json:"challenge"`
	StartedAt                    int64  `json:"startedAt"`
	CompletedAt                  int64  `json:"completedAt"`
	RequestSent                  bool   `json:"requestSent"`
	ResponseReceived             bool   `json:"responseReceived"`
	IntendedInboundObserved      bool   `json:"intendedInboundObserved"`
	IntendedSinkObserved         bool   `json:"intendedSinkObserved"`
	WrongSinkUntouched           bool   `json:"wrongSinkUntouched"`
	OriginalDestinationPreserved bool   `json:"originalDestinationPreserved"`
	SourcePreserved              bool   `json:"sourcePreserved"`
	ManagementPreserved          bool   `json:"managementPreserved"`
	BoundedFlowExpired           bool   `json:"boundedFlowExpired"`
}

type InterceptionProbeProviderV1 interface {
	ProviderID() string
	SupportsInterceptionProbeV1(InterceptionProbeTargetV1) bool
	ProbeInterceptionV1(context.Context, InterceptionProbeRequestV1) (InterceptionProbeObservationV1, error)
}

func (t InterceptionProbeTargetV1) Validate() error {
	if t.Schema != InterceptionProbeTargetSchemaV1 ||
		!healthToken(t.ProviderID, 128) || !healthToken(t.ResourceID, 256) || !healthToken(t.EndpointID, 256) ||
		(t.Kind != "REDIRECT" && t.Kind != "TPROXY") || (t.Network != "tcp" && t.Network != "udp") ||
		(t.AddressFamily != "ipv4" && t.AddressFamily != "ipv6") ||
		!healthToken(t.IngressScopeProviderID, 128) || !healthToken(t.IngressScopeID, 256) ||
		!healthDigest(t.IngressScopeRevision) || !healthDigest(t.InterceptionFactRevision) ||
		!healthDigest(t.RuntimeRevision) || !healthDigest(t.ListenerRevision) ||
		!healthDigest(t.ManagedCandidateRevision) || !healthDigest(t.ManagementExclusionRevision) ||
		!healthDigest(t.HealthTransactionRevision) ||
		t.Kind == "REDIRECT" && (t.Network != "tcp" || t.PolicyRoutingRevision != "") ||
		t.Kind == "TPROXY" && !healthDigest(t.PolicyRoutingRevision) {
		return errors.New("interception_probe_target_v1_invalid")
	}
	return nil
}

func (t InterceptionProbeTargetV1) Revision() string {
	return hostresources.Revision(t)
}

func (r InterceptionProbeRequestV1) Validate() error {
	if r.Target.Validate() != nil || !healthToken(r.OperationID, 128) ||
		!healthDigest(r.MarkerRevision) || !healthToken(r.RuntimeGeneration, 128) ||
		!healthToken(r.Challenge, 128) || r.MarkerAt <= 0 || r.DeadlineAt <= r.MarkerAt ||
		r.DeadlineAt-r.MarkerAt > 30 {
		return errors.New("interception_probe_request_v1_invalid")
	}
	return nil
}

func (o InterceptionProbeObservationV1) Validate(request InterceptionProbeRequestV1) error {
	if request.Validate() != nil || o.Schema != InterceptionProbeObservationSchemaV1 ||
		!healthToken(o.ProviderID, 128) || o.TargetRevision != request.Target.Revision() ||
		o.OperationID != request.OperationID || o.MarkerRevision != request.MarkerRevision ||
		o.RuntimeGeneration != request.RuntimeGeneration || o.Challenge != request.Challenge ||
		o.StartedAt <= request.MarkerAt || o.CompletedAt < o.StartedAt || o.CompletedAt > request.DeadlineAt ||
		!o.RequestSent || !o.ResponseReceived || !o.IntendedInboundObserved || !o.IntendedSinkObserved ||
		!o.WrongSinkUntouched || !o.OriginalDestinationPreserved || !o.SourcePreserved || !o.ManagementPreserved ||
		request.Target.Network == "udp" && !o.BoundedFlowExpired {
		return errors.New("interception_probe_observation_v1_invalid")
	}
	return nil
}

func healthDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' && r < 'a' || r > 'f' {
			return false
		}
	}
	return true
}
