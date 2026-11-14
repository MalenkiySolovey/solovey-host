package deploymentidentity_test

import (
	"encoding/json"
	identity "github.com/MalenkiySolovey/solovey-host/componenthost/deploymentidentity"
	"strings"
	"testing"
)

func TestExpectedOwnerIsAValidatedValue(t *testing.T) {
	value := identity.ExpectedApplicationOwnerV1{Schema: identity.ExpectedApplicationOwnerSchemaV1,
		ContractRevision: strings.Repeat("a", 64), InstanceID: "00112233-4455-4677-8899-aabbccddeeff",
		SourceRevision: "src-" + strings.Repeat("b", 64), ArtifactRevision: "art-" + strings.Repeat("c", 64),
		DeploymentID: "dep-" + strings.Repeat("d", 64), RuntimeRootBindingRevision: strings.Repeat("e", 64),
		ServiceIdentity: "solovey-ui", ExecutablePath: "/usr/lib/solovey-ui/solovey-ui", ExecutableSHA256: strings.Repeat("f", 64)}
	if err := value.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var restored identity.ExpectedApplicationOwnerV1
	if err = json.Unmarshal(data, &restored); err != nil || restored != value || !restored.Valid() {
		t.Fatalf("value serialization changed: %s, %v", data, err)
	}
	for name, mutate := range map[string]func(*identity.ExpectedApplicationOwnerV1){
		"unknown schema":                    func(v *identity.ExpectedApplicationOwnerV1) { v.Schema = "" },
		"missing installed proof reference": func(v *identity.ExpectedApplicationOwnerV1) { v.ContractRevision = "" },
		"missing runtime root reference":    func(v *identity.ExpectedApplicationOwnerV1) { v.RuntimeRootBindingRevision = "" },
		"relative executable":               func(v *identity.ExpectedApplicationOwnerV1) { v.ExecutablePath = "bin/server" },
		"noncanonical executable":           func(v *identity.ExpectedApplicationOwnerV1) { v.ExecutablePath = "/usr/../bin/server" },
		"malformed source":                  func(v *identity.ExpectedApplicationOwnerV1) { v.SourceRevision = "src-main" },
		"invalid identity":                  func(v *identity.ExpectedApplicationOwnerV1) { v.ServiceIdentity = "server\nshell" },
	} {
		t.Run(name, func(t *testing.T) {
			v := value
			mutate(&v)
			if v.Valid() {
				t.Fatal("malformed expected-owner value accepted")
			}
		})
	}
}
