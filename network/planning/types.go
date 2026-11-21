// Package planning provides typed inputs and views for the existing network
// resource coordinator. Values grant no allocation or execution authority.
package planning

import (
	"fmt"

	"github.com/MalenkiySolovey/solovey-host/hostref"
	networkintent "github.com/MalenkiySolovey/solovey-host/network/intent"
)

const Schema = "solovey-ui/network-plan/v1"
const ProtectionAllocation = networkintent.ProtectionAllocation
const ProtectionResourceRef = networkintent.ProtectionResourceRef
const ProtectionFamily = networkintent.ProtectionFamily
const ProtectionTable = networkintent.ProtectionTable
const ProtectionTableCLI = networkintent.ProtectionTableCLI
const ProtectionEndpointMode = networkintent.ProtectionEndpointMode

type Reason string

const (
	InvalidIntent         Reason = "INVALID_INTENT"
	StalePlan             Reason = "STALE_PLAN"
	ResourceConflict      Reason = "RESOURCE_CONFLICT"
	ResourceGone          Reason = "RESOURCE_GONE"
	EndpointChanged       Reason = "ENDPOINT_CHANGED"
	CapabilityUnavailable Reason = "CAPABILITY_UNAVAILABLE"
	ObservationUnknown    Reason = "OBSERVATION_UNKNOWN"
	ManagementUnknown     Reason = "MANAGEMENT_CONTINUITY_UNKNOWN"
	ManagementBlocked     Reason = "MANAGEMENT_CONTINUITY_BLOCKED"
	CandidateChanged      Reason = "CANDIDATE_CHANGED"
	AllocationUnavailable Reason = "ALLOCATION_UNAVAILABLE"
)

type Conflict struct {
	Reason      Reason `json:"reason"`
	ResourceRef string `json:"resourceRef"`
	Allocation  string `json:"allocation,omitempty"`
}

func (c *Conflict) Error() string { return fmt.Sprintf("network plan %s: %s", c.Reason, c.ResourceRef) }

type ResourceBinding = networkintent.ResourceBinding
type Intent = networkintent.Intent
type ProtectionEnforcementIntent = networkintent.ProtectionEnforcementIntent
type ZapretPacketPathIntent = networkintent.ZapretPacketPathIntent

const (
	CapabilityAvailable        = "AVAILABLE"
	CapabilityUnavailableState = "UNAVAILABLE"
	CapabilityUnknown          = "UNKNOWN"
)

type Ownership struct {
	HostRef      hostref.Ref `json:"hostRef"`
	Owner        string      `json:"owner"`
	ResourceRef  string      `json:"resourceRef"`
	PlanRevision string      `json:"planRevision"`
	Kind         string      `json:"kind"`
	Purpose      string      `json:"purpose"`
	Allocation   string      `json:"allocation"`
}

type Allocation struct {
	Ownership
	Proven           bool `json:"proven"`
	IdentityMismatch bool `json:"identityMismatch,omitempty"`
}

type Continuity struct {
	ResourceRef      string `json:"resourceRef"`
	EndpointRevision string `json:"endpointRevision"`
	Status           string `json:"status"`
}

// CapabilityFact is a projection of an existing platform observation. It is
// never an execution grant or a substitute for current owner admission.
type CapabilityFact struct {
	Availability string `json:"availability"`
	Backend      string `json:"backend"`
	Reason       string `json:"reason"`
}

type Facts struct {
	HostRef            hostref.Ref       `json:"hostRef"`
	Revision           string            `json:"revision"`
	CapabilityRevision string            `json:"capabilityRevision"`
	Capability         CapabilityFact    `json:"capability"`
	InventoryComplete  bool              `json:"inventoryComplete"`
	Resources          []ResourceBinding `json:"resources"`
	Allocations        []Allocation      `json:"allocations"`
	Continuity         []Continuity      `json:"continuity"`
}

type DriftKind string

const (
	Missing                   DriftKind = "MISSING"
	Mismatch                  DriftKind = "MISMATCH"
	Stale                     DriftKind = "STALE"
	UnexpectedManagedResource DriftKind = "UNEXPECTED_MANAGED_RESOURCE"
	DependencyUnavailable     DriftKind = "DEPENDENCY_UNAVAILABLE"
	Unknown                   DriftKind = "UNKNOWN"
)

type Drift struct {
	Kind        DriftKind `json:"kind"`
	ResourceRef string    `json:"resourceRef"`
	Repairable  bool      `json:"repairable"`
}

type RollbackIntent struct {
	HostRef                                                                                   hostref.Ref
	Owner, ResourceRef, CurrentRevision, CurrentSemantic, CurrentMembership, CheckpointDigest string
}
