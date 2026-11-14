package fallbacktargets

import (
	"errors"
)

const TargetReferenceSchemaV2 = "solovey-ui/fallback-target-reference/v2"

const MaxOpaqueIDLengthV2 = 128

type FallbackTargetReferenceV2 struct {
	Schema                 string `json:"schema"`
	ProviderID             string `json:"providerId"`
	TargetID               string `json:"targetId"`
	PublishRevision        string `json:"publishRevision"`
	ContentDigest          string `json:"contentDigest"`
	EndpointID             string `json:"endpointId"`
	EndpointRevision       string `json:"endpointRevision"`
	ProviderHealthRevision string `json:"providerHealthRevision"`
	CapacityRevision       string `json:"capacityRevision"`
	ProviderRevision       string `json:"providerRevision"`
}

func (r FallbackTargetReferenceV2) Validate() error {
	if r.Schema != TargetReferenceSchemaV2 || !validOpaqueID(r.ProviderID, MaxOpaqueIDLengthV2) ||
		!validOpaqueID(r.TargetID, MaxOpaqueIDLengthV2) || !validOpaqueID(r.PublishRevision, MaxOpaqueIDLengthV2) ||
		!isSHA256(r.ContentDigest) || !validOpaqueID(r.EndpointID, MaxOpaqueIDLengthV2) ||
		!isSHA256(r.EndpointRevision) || !isSHA256(r.ProviderHealthRevision) ||
		!isSHA256(r.CapacityRevision) || !validOpaqueID(r.ProviderRevision, MaxOpaqueIDLengthV2) {
		return errors.New("fallback_target_reference_v2_invalid")
	}
	return nil
}
