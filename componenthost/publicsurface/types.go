// Package publicsurface defines public request context and bounded, classified
// observations. Registration, request identity and delivery remain owner adapters.
package publicsurface

// Context identifies the existing managed web surface.
type Context struct{ AdminBasePath string }

type Observation struct {
	ResourceID     string `json:"resourceId"`
	ResourceKind   string `json:"resourceKind"`
	ComponentID    string `json:"componentId,omitempty"`
	SourceIP       string `json:"sourceIp"`
	MethodClass    string `json:"methodClass"`
	PathClass      string `json:"pathClass"`
	StatusClass    string `json:"statusClass"`
	UserAgentClass string `json:"userAgentClass"`
	BytesClass     string `json:"bytesClass"`
	DurationClass  string `json:"durationClass"`
	RateLimited    bool   `json:"rateLimited"`
	ObservedAt     int64  `json:"observedAt"`
}
