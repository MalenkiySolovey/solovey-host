package operationref

import "strings"

type Outcome string

const (
	OutcomePending          Outcome = "PENDING"
	OutcomeCommitted        Outcome = "COMMITTED"
	OutcomeFailed           Outcome = "FAILED"
	OutcomeRolledBack       Outcome = "ROLLED_BACK"
	OutcomeRecoveryRequired Outcome = "RECOVERY_REQUIRED"
)

// Problem carries only owner-controlled codes and references. It intentionally
// has no arbitrary payload/error field that could copy secrets into reports.
type Problem struct {
	Owner            string `json:"owner"`
	ResourceID       string `json:"resourceId"`
	Phase            string `json:"phase"`
	Code             string `json:"code"`
	RecoveryRequired bool   `json:"recoveryRequired"`
}

// Change projects the semantic lifecycle; it never interprets owner payloads
// or decides compensation. ExpectedRevision keeps the owner's existing binding
// (for example a restore rehearsal), not a new cross-owner authority token.
type Change struct {
	Reference
	Revision         uint64   `json:"revision"`
	ExpectedRevision string   `json:"expectedRevision,omitempty"`
	Phase            string   `json:"phase"`
	OwnerPhase       string   `json:"ownerPhase"`
	Outcome          Outcome  `json:"outcome"`
	CheckpointRef    string   `json:"checkpointRef,omitempty"`
	RollbackVerified bool     `json:"rollbackVerified"`
	RecoveryRequired bool     `json:"recoveryRequired"`
	Problem          *Problem `json:"problem,omitempty"`
}

func (c Change) WithProblem(code string) Change {
	if code != "" {
		if len(code) > 96 || strings.Trim(code, "abcdefghijklmnopqrstuvwxyz0123456789_") != "" {
			code = "operation_reason_invalid"
		}
		c.Problem = &Problem{Owner: c.Owner, ResourceID: c.ResourceID, Phase: c.Phase, Code: code, RecoveryRequired: c.RecoveryRequired}
	}
	return c
}

type Progress struct {
	Completed int64  `json:"completed"`
	Total     int64  `json:"total"`
	Unit      string `json:"unit"`
}

// Job is a finite owner-operation read envelope, not another durable job or
// scheduler. Nil progress/deadline/cancel request means no such owner truth.
type Job struct {
	Reference
	JobID            string    `json:"jobId"`
	Revision         uint64    `json:"revision"`
	Phase            string    `json:"phase"`
	ExpectedRevision string    `json:"expectedRevision,omitempty"`
	Progress         *Progress `json:"progress,omitempty"`
	Deadline         int64     `json:"deadline,omitempty"`
	CancelRequested  *bool     `json:"cancelRequested,omitempty"`
	CancelSupported  bool      `json:"cancelSupported"`
	CriticalSection  bool      `json:"criticalSection"`
	Outcome          Outcome   `json:"outcome"`
	RecoveryRequired bool      `json:"recoveryRequired"`
	Problem          *Problem  `json:"problem,omitempty"`
	ResultRef        string    `json:"resultRef,omitempty"`
}

func ProjectJob(change Change, progress *Progress, criticalSection bool) Job {
	correlation := change.Reference.Correlation()
	result := Job{Reference: correlation.Reference, JobID: correlation.JobID,
		Revision: change.Revision, Phase: change.OwnerPhase, ExpectedRevision: change.ExpectedRevision,
		CriticalSection: criticalSection, Outcome: change.Outcome, RecoveryRequired: change.RecoveryRequired, Problem: change.Problem}
	if progress != nil && progress.Unit == "bytes" && progress.Total > 0 && progress.Completed >= 0 && progress.Completed <= progress.Total {
		copy := *progress
		result.Progress = &copy
	}
	if change.Outcome != OutcomePending {
		result.ResultRef = change.OperationID
	}
	return result
}
