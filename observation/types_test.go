package observation

import "testing"

func TestSamplingVocabularyPreservesExistingBucketsAndMetrics(t *testing.T) {
	for got, want := range map[ObservabilityBucket]string{
		ObservabilityBucket2s: "2s", ObservabilityBucket30s: "30s",
		ObservabilityBucket1m: "1m", ObservabilityBucket5m: "5m",
	} {
		if string(got) != want {
			t.Fatal("sampling bucket changed")
		}
	}
	for got, want := range map[ObservabilityMetric]string{
		ObservabilityMetricCPU: "cpu", ObservabilityMetricRAM: "ram",
		ObservabilityMetricNetIn: "net_in", ObservabilityMetricNetOut: "net_out",
	} {
		if string(got) != want {
			t.Fatal("sampling metric changed")
		}
	}
}
