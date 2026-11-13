package health

import (
	"github.com/MalenkiySolovey/solovey-host/hostref"
	"testing"
	"time"
)

func TestEvidenceRequiresExactOwnerRevisionAndFreshProof(t *testing.T) {
	now := time.Unix(1800000000, 0)
	revision := Revision("publication:generation:2")
	result := WithEvidence(Result{ResourceID: "component:fallback-html:site:1", Status: StatusOK, Check: "runtime", FactCode: "runtime_ready"}, "fallback-html", RuntimeReady, "fallback_runtime", revision, now, 30*time.Second)
	req := Requirement{Owner: "fallback-html", Source: "fallback_runtime", ProofKind: RuntimeReady, DesiredRevision: revision, MaxAge: 30 * time.Second}
	if !result.Satisfies(req, now) {
		t.Fatal("exact proof rejected")
	}
	for _, mutate := range []func(*Evidence){func(e *Evidence) { e.HostRef = hostref.Ref{} }, func(e *Evidence) { e.Owner = "other" }, func(e *Evidence) { e.ObservedRevision = Revision("stale") }, func(e *Evidence) { e.ProofKind = ConfigValid }, func(e *Evidence) { e.ObservedAt = now.Add(time.Second).Unix() }, func(e *Evidence) { e.MaxAgeSeconds = 61 }, func(e *Evidence) { e.EvidenceRef = "other" }} {
		copy := result
		e := *result.Evidence
		mutate(&e)
		copy.Evidence = &e
		if copy.Satisfies(req, now) {
			t.Fatalf("invalid proof accepted %+v", e)
		}
	}
	if result.Satisfies(req, now.Add(31*time.Second)) {
		t.Fatal("expired proof accepted")
	}
	if (Result{ResourceID: result.ResourceID, Status: StatusOK}).Satisfies(req, now) {
		t.Fatal("legacy observation became proof")
	}
}
