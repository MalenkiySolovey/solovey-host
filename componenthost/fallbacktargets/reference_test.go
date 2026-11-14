package fallbacktargets_test

import (
	"encoding/json"
	target "github.com/MalenkiySolovey/solovey-host/componenthost/fallbacktargets"
	"strings"
	"testing"
)

func TestFallbackReferenceRetainsAllRevisionBindings(t *testing.T) {
	ref := target.FallbackTargetReferenceV2{Schema: target.TargetReferenceSchemaV2, ProviderID: "fallback-html", TargetID: "site:default", PublishRevision: "publication:1",
		ContentDigest: strings.Repeat("a", 64), EndpointID: "endpoint:local", EndpointRevision: strings.Repeat("b", 64), ProviderHealthRevision: strings.Repeat("c", 64), CapacityRevision: strings.Repeat("d", 64), ProviderRevision: "provider:1"}
	if err := ref.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	var restored target.FallbackTargetReferenceV2
	if err = json.Unmarshal(data, &restored); err != nil || restored != ref {
		t.Fatalf("reference representation changed: %v", err)
	}
	for name, mutate := range map[string]func(*target.FallbackTargetReferenceV2){
		"schema":      func(r *target.FallbackTargetReferenceV2) { r.Schema = "" },
		"provider":    func(r *target.FallbackTargetReferenceV2) { r.ProviderID = "../provider" },
		"publication": func(r *target.FallbackTargetReferenceV2) { r.PublishRevision = "" },
		"endpoint":    func(r *target.FallbackTargetReferenceV2) { r.EndpointRevision = "stale" },
		"health":      func(r *target.FallbackTargetReferenceV2) { r.ProviderHealthRevision = "" },
		"capacity":    func(r *target.FallbackTargetReferenceV2) { r.CapacityRevision = "" },
		"bounds":      func(r *target.FallbackTargetReferenceV2) { r.TargetID = strings.Repeat("x", 129) },
	} {
		t.Run(name, func(t *testing.T) {
			v := ref
			mutate(&v)
			if v.Validate() == nil {
				t.Fatal("malformed reference accepted")
			}
		})
	}
}
