// Package observation defines neutral sampling and history projections.
// Collection, allocation and retention policy remain with their existing owners.
package observation

type ObservabilityBucket string
type ObservabilityMetric string

const ()

type ObservabilitySample struct {
	DateTime int64                  `json:"dateTime"`
	CPU      float64                `json:"cpu"`
	Memory   map[string]interface{} `json:"memory"`
	Network  map[string]interface{} `json:"network"`
}
type CoreSample struct {
	DateTime int64                  `json:"dateTime"`
	Core     map[string]interface{} `json:"core"`
}
type ObservabilityMetricSample struct {
	DateTime int64   `json:"dateTime"`
	Value    float64 `json:"value"`
}
