package deploymentidentity

import (
	"errors"
)

const ExpectedApplicationOwnerSchemaV1 = "solovey-ui/expected-application-owner/v1"

type ExpectedApplicationOwnerV1 struct {
	Schema                     string `json:"schema"`
	ContractRevision           string `json:"contractRevision"`
	InstanceID                 string `json:"instanceId"`
	SourceRevision             string `json:"sourceRevision"`
	ArtifactRevision           string `json:"artifactRevision"`
	DeploymentID               string `json:"deploymentId"`
	RuntimeRootBindingRevision string `json:"runtimeRootBindingRevision"`
	ServiceIdentity            string `json:"serviceIdentity"`
	ExecutablePath             string `json:"executablePath"`
	ExecutableSHA256           string `json:"executableSha256"`
	ProcessUID                 uint32 `json:"processUid"`
	ProcessGID                 uint32 `json:"processGid"`
}

func (e ExpectedApplicationOwnerV1) Validate() error {
	if e.Schema != ExpectedApplicationOwnerSchemaV1 || !revisionPattern.MatchString(e.ContractRevision) ||
		!uuidPattern.MatchString(e.InstanceID) || !sourcePattern.MatchString(e.SourceRevision) ||
		!artifactPattern.MatchString(e.ArtifactRevision) || !deployPattern.MatchString(e.DeploymentID) ||
		!revisionPattern.MatchString(e.RuntimeRootBindingRevision) || !identityPattern.MatchString(e.ServiceIdentity) ||
		!canonicalAbsolute(e.ExecutablePath) || !revisionPattern.MatchString(e.ExecutableSHA256) {
		return errors.New("expected application owner is malformed")
	}
	return nil
}

func (e ExpectedApplicationOwnerV1) Valid() bool { return e.Validate() == nil }
