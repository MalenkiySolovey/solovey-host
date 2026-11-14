package planning

import networkintent "github.com/MalenkiySolovey/solovey-host/network/intent"

// ProtectionPlan exposes only copied read views. The concrete coordinator
// retains the immutable plan and rejects foreign handles during acceptance.
type ProtectionPlan interface {
	Revision() string
	Intent() Intent
	Resources() []Ownership
	Enforcement() networkintent.ProtectionEnforcementIntent
}

// Accepted is an owner-created candidate receipt. Reading its digest does not
// recreate it; the coordinator verifies the concrete handle and current facts.
type Accepted interface {
	PlanRevision() string
	CandidateDigest() string
}
