package operationref

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/MalenkiySolovey/solovey-host/hostref"
)

// Reference correlates projections without creating another durable operation.
// IDs are derived from an owner namespace and its existing opaque operation ID.
type Reference struct {
	CorrelationID string      `json:"correlationId"`
	ChangeID      string      `json:"changeId"`
	HostRef       hostref.Ref `json:"hostRef"`
	Owner         string      `json:"owner"`
	ResourceID    string      `json:"resourceId"`
	OperationID   string      `json:"operationId"`
}

func NewReference(owner, resourceID, operationID string) Reference {
	sum := sha256.Sum256([]byte(owner + "\x00" + operationID))
	id := hex.EncodeToString(sum[:24])
	if !ValidToken(owner, 64) || !ValidToken(resourceID, 256) || !ValidToken(operationID, 96) {
		owner, resourceID, operationID = "unavailable", "unavailable", "unavailable"
	}
	return Reference{CorrelationID: "operation:" + id, ChangeID: "change:" + id,
		HostRef: hostref.Local(), Owner: owner, ResourceID: resourceID, OperationID: operationID}
}

type Correlation struct {
	Reference
	JobID string `json:"jobId"`
}

func (r Reference) Correlation() Correlation {
	id := strings.TrimPrefix(r.CorrelationID, "operation:")
	if !strings.HasPrefix(r.CorrelationID, "operation:") || len(id) != 48 || strings.Trim(id, "0123456789abcdef") != "" ||
		r.ChangeID != "change:"+id || !ValidToken(r.Owner, 64) || !ValidToken(r.ResourceID, 256) ||
		!ValidToken(r.OperationID, 96) || !r.HostRef.IsLocal() {
		r = NewReference(r.Owner, r.ResourceID, r.OperationID)
		id = strings.TrimPrefix(r.CorrelationID, "operation:")
	}
	return Correlation{Reference: r, JobID: "job:" + id}
}

// ValidToken validates the bounded identity vocabulary shared by owner projections.
func ValidToken(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.Trim(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._:@+-") == ""
}
