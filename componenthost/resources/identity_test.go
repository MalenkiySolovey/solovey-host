package resources

import (
	"encoding/json"
	"github.com/MalenkiySolovey/solovey-host/hostref"
	"testing"
)

func TestConfiguredIdentityRemainsUnknownAndScoped(t *testing.T) {
	r := ConfiguredResource{ID: "component:fallback-html:site:7", Owner: "fallback-html", Source: "component", Revision: Revision("saved-target")}
	identity := LocalConfiguredIdentity(r)
	if identity == nil || !identity.ValidFor(r) {
		t.Fatal("configured tuple rejected")
	}
	var decoded ResourceIdentityV1
	b, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &decoded); err != nil || !decoded.ValidFor(r) {
		t.Fatal("JSON identity changed", err)
	}
	mutations := []func(*ResourceIdentityV1){func(x *ResourceIdentityV1) { x.Host = hostref.Ref{} }, func(x *ResourceIdentityV1) { x.Owner = "other" }, func(x *ResourceIdentityV1) { x.ResourceID += "x" }, func(x *ResourceIdentityV1) { x.DesiredRevision = Revision("changed") }, func(x *ResourceIdentityV1) { x.Source = "other" }, func(x *ResourceIdentityV1) { x.ObservedRevision = x.DesiredRevision }, func(x *ResourceIdentityV1) { x.ObservationStatus = "READY" }}
	for _, mutate := range mutations {
		x := *identity
		mutate(&x)
		if x.ValidFor(r) {
			t.Fatalf("foreign or observed identity accepted: %+v", x)
		}
	}
	for _, token := range []string{"", "wrong/name", " space", "bad?query", "bad\nline", "слово"} {
		bad := r
		bad.ID = token
		if LocalConfiguredIdentity(bad) != nil {
			t.Fatalf("unsafe token accepted %q", token)
		}
	}
}
func TestListenCompatibility(t *testing.T) {
	for _, c := range []struct {
		input, value string
		class        ListenClass
	}{{"", "*", ListenWildcard}, {"[::1]", "::1", ListenLoopback}, {"127.0.0.1", "127.0.0.1", ListenLoopback}, {"[::]", "::", ListenIPv6Wildcard}, {"EXAMPLE.COM.", "example.com", ListenHostname}} {
		got := NormalizeListen(c.input)
		if got.Value != c.value || got.Class != c.class {
			t.Fatalf("%q: %+v", c.input, got)
		}
	}
}
