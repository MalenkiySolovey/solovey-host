package health

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

type Status string

const (
	StatusOK                Status = "ok"
	StatusDegraded          Status = "degraded"
	StatusMissingCapability Status = "missing_capability"
)

// Result intentionally contains only stable codes: health output is safe for
// diagnostics and recovery artifacts and cannot disclose listener paths.
type Result struct {
	ResourceID string    `json:"resourceId"`
	Status     Status    `json:"status"`
	Check      string    `json:"check"`
	FactCode   string    `json:"factCode"`
	Evidence   *Evidence `json:"evidence,omitempty"`
}

type ProofKind string

const (
	ConfigValid       ProofKind = "CONFIG_VALID"
	Applied           ProofKind = "APPLIED"
	RuntimeReady      ProofKind = "RUNTIME_READY"
	ProbeOK           ProofKind = "PROBE_OK"
	DataPlaneVerified ProofKind = "DATA_PLANE_VERIFIED"
	EndToEndVerified  ProofKind = "END_TO_END_VERIFIED"
)

// Evidence proves a specific proposition. Proof kinds have no numeric ordering.
// DesiredRevision is the consumer binding; ObservedRevision is supplied only by
// the semantic checker. An observation is never a repair or authorization grant.
type Evidence struct {
	HostRef          hostref.Ref `json:"hostRef"`
	Owner            string      `json:"owner"`
	ResourceID       string      `json:"resourceId"`
	ProofKind        ProofKind   `json:"proofKind"`
	Source           string      `json:"source"`
	DesiredRevision  string      `json:"desiredRevision,omitempty"`
	ObservedRevision string      `json:"observedRevision"`
	EvidenceRevision string      `json:"evidenceRevision"`
	ObservedAt       int64       `json:"observedAt"`
	MaxAgeSeconds    int64       `json:"maxAgeSeconds"`
	EvidenceRef      string      `json:"evidenceRef"`
}

type Requirement struct {
	Owner           string
	Source          string
	ProofKind       ProofKind
	DesiredRevision string
	MaxAge          time.Duration
}

func Revision(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func WithEvidence(result Result, owner string, proof ProofKind, source, observedRevision string, now time.Time, maxAge time.Duration) Result {
	evidence := &Evidence{HostRef: hostref.Local(), Owner: owner, ResourceID: result.ResourceID,
		ProofKind: proof, Source: source, ObservedRevision: observedRevision, ObservedAt: now.Unix(), MaxAgeSeconds: int64(maxAge / time.Second)}
	evidence.EvidenceRevision = Revision(owner + "\x00" + result.ResourceID + "\x00" + string(proof) + "\x00" + source + "\x00" +
		observedRevision + "\x00" + now.UTC().Format(time.RFC3339Nano) + "\x00" + string(result.Status) + "\x00" + result.FactCode)
	evidence.EvidenceRef = "health:" + evidence.EvidenceRevision
	result.Evidence = evidence
	return result
}

func (r Result) Satisfies(requirement Requirement, now time.Time) bool {
	e := r.Evidence
	return r.Status == StatusOK && ValidEvidence(r.ResourceID, e) &&
		healthToken(requirement.Owner, 64) && healthToken(requirement.Source, 64) && proofKind(requirement.ProofKind) &&
		digest(requirement.DesiredRevision) && requirement.MaxAge > 0 && requirement.MaxAge <= time.Minute &&
		e.Owner == requirement.Owner && e.Source == requirement.Source && e.ProofKind == requirement.ProofKind &&
		e.ObservedRevision == requirement.DesiredRevision && e.ObservedAt <= now.Unix() &&
		now.Unix()-e.ObservedAt <= e.MaxAgeSeconds && now.Sub(time.Unix(e.ObservedAt, 0)) <= requirement.MaxAge
}

func ValidEvidence(resourceID string, e *Evidence) bool {
	return e != nil && e.HostRef.IsLocal() && e.ResourceID == resourceID && healthToken(e.Owner, 64) &&
		healthToken(e.Source, 64) && proofKind(e.ProofKind) && digest(e.ObservedRevision) && digest(e.EvidenceRevision) &&
		(e.DesiredRevision == "" || digest(e.DesiredRevision)) && e.ObservedAt > 0 && e.MaxAgeSeconds > 0 && e.MaxAgeSeconds <= 60 &&
		e.EvidenceRef == "health:"+e.EvidenceRevision
}

func digest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func proofKind(kind ProofKind) bool {
	switch kind {
	case ConfigValid, Applied, RuntimeReady, ProbeOK, DataPlaneVerified, EndToEndVerified:
		return true
	default:
		return false
	}
}

func healthToken(value string, limit int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > limit || strings.ContainsAny(value, "/\\?#&={}[]<>\"'\r\n\t ") {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			strings.ContainsRune("._:@+-", r) {
			continue
		}
		return false
	}
	return true
}
