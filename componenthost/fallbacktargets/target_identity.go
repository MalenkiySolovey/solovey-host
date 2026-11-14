package fallbacktargets

type Readiness string

const (
	ReadinessReady    Readiness = "READY"
	ReadinessNotReady Readiness = "NOT_READY"
	ReadinessStale    Readiness = "STALE"
	ReadinessUnknown  Readiness = "UNKNOWN"
)

type TargetIdentity struct {
	ProviderID string `json:"providerId"`
	TargetID   string `json:"targetId"`
}
