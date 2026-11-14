package resources_test

import (
	r "github.com/MalenkiySolovey/solovey-host/componenthost/resources"
	"strings"
	"testing"
	"time"
)

func TestConfiguredInventoryValuesKeepUnknownAndIdentityFences(t *testing.T) {
	now := time.Unix(7000, 0).UTC()
	value := r.ProtectableResource{ID: "core:panel:web", Owner: "core", Source: "settings", Kind: "panel", Protocol: "tcp", Listen: "0.0.0.0", Port: 2095,
		Capabilities: r.ProtectableResourceCapabilities{Known: true, ConfigRevision: strings.Repeat("a", 64)}}
	value.Identity = r.LocalConfiguredIdentity(value.ConfiguredResource())
	if !value.Identity.ValidFor(value.ConfiguredResource()) {
		t.Fatal("configured identity rejected")
	}
	foreign := value
	foreign.Owner = "foreign"
	if value.Identity.ValidFor(foreign.ConfiguredResource()) {
		t.Fatal("foreign owner accepted")
	}
	endpoint := r.BuildEndpointFact(value, r.NetworkTCP, now)
	if endpoint.AuthenticationExpected != r.CapabilityUnknown || endpoint.FallbackSupported != r.CapabilityValue("") {
		t.Fatal("configuration gained capability authority")
	}
	endpoint.FallbackSupported = r.CapabilityValue("guess")
	endpoint = r.NormalizeEndpointFact(value, endpoint, now)
	if endpoint.FallbackSupported != r.CapabilityUnknown {
		t.Fatal("unsupported capability became known")
	}
	before := r.Fingerprint(value)
	copy := value
	copy.Name = "new label"
	copy.Identity = nil
	if r.Fingerprint(copy) != before {
		t.Fatal("read metadata changed configured fingerprint")
	}
	copy.Capabilities.ConfigRevision = strings.Repeat("b", 64)
	if r.Fingerprint(copy) == before {
		t.Fatal("revision drift was hidden")
	}
	keys, ok := r.DeterministicConfiguredEndpointKeys(value)
	if !ok || len(keys) != 1 || keys[0].Port != 2095 {
		t.Fatalf("configured keys: %#v %v", keys, ok)
	}
	value.Listen = "::"
	keys, ok = r.DeterministicConfiguredEndpointKeys(value)
	if !ok || len(keys) != 2 {
		t.Fatalf("wildcard management preservation lost a family: %#v", keys)
	}
	value.ListenIntents = []r.ConfiguredListenIntentV1{r.BuildConfiguredListenIntent(value)}
	value.ListenIntents[0].ConfigurationRevision = strings.Repeat("c", 64)
	if _, ok = r.ConfiguredListenIntents(value); ok {
		t.Fatal("stale configured intent accepted")
	}
}

func TestManagementValuesRejectAmbiguousAndStaleEvidence(t *testing.T) {
	now := time.Unix(7000, 0).UTC()
	value := r.ProtectableResource{ID: "core:panel:web", Owner: "core", Source: "settings", Protocol: "tcp", Listen: "192.0.2.10", Port: 2095,
		Capabilities: r.ProtectableResourceCapabilities{ConfigRevision: strings.Repeat("a", 64)}}
	endpoint := r.ManagementEndpointFromResource(value, r.ManagementPanel, now)
	if !r.ManagementEndpointCurrent(endpoint, now) {
		t.Fatalf("valid configured management fact rejected: %#v", endpoint)
	}
	for name, mutate := range map[string]func(*r.ManagementEndpointV1){
		"expired":            func(e *r.ManagementEndpointV1) { e.ExpiresAt = now.Unix() },
		"unknown family":     func(e *r.ManagementEndpointV1) { e.Family = r.AddressFamilyUnknown },
		"ambiguous":          func(e *r.ManagementEndpointV1) { e.ReasonCodes = []string{"owner_ambiguous"} },
		"unknown revision":   func(e *r.ManagementEndpointV1) { e.ConfigurationRevision = "" },
		"mixed ssh evidence": func(e *r.ManagementEndpointV1) { e.ServiceKind = r.ManagementSSH; e.ObservedListener = true },
	} {
		t.Run(name, func(t *testing.T) {
			v := endpoint
			mutate(&v)
			if r.ManagementEndpointCurrent(v, now) {
				t.Fatal("unsafe management fact accepted")
			}
		})
	}
	path := r.RecoveryPathV1{Schema: r.RecoveryPathSchemaV1, ID: "recovery:panel", Kind: "PANEL", EndpointID: endpoint.ID, PrincipalID: "admin", VerificationMethod: "fresh_panel_login",
		VerifiedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(), IndependenceClass: "independent", VerificationState: "verified",
		SourceRevision: strings.Repeat("a", 64), ConfigurationRevision: strings.Repeat("a", 64)}
	if !r.RecoveryPathFresh(path, now) {
		t.Fatal("fresh independent recovery rejected")
	}
	path.VerificationMethod = "existing_connection"
	if r.RecoveryPathFresh(path, now) {
		t.Fatal("existing session gained recovery authority")
	}
}
