// Package auditrecord defines the existing neutral audit event value.
// Validation, redaction and persistence belong to the receiving audit owner.
package auditrecord

type Event struct {
	Actor     string
	Event     string
	Resource  string
	Severity  string
	IP        string
	UserAgent string
	Details   map[string]any
}
