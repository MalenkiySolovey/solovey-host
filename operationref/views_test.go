package operationref

import "testing"

func TestReadProblemRejectsRawOrUnboundedOwnerPayload(t *testing.T) {
	change := Change{Reference: NewReference("owner", "resource", "operation"), OwnerPhase: "ROLLBACK_FAILED", Phase: "RECOVERY_REQUIRED", RecoveryRequired: true}
	for _, code := range []string{"private token=secret/path\n", "UPPERCASE", string(make([]byte, 97))} {
		result := change.WithProblem(code)
		if result.Problem == nil || result.Problem.Code != "operation_reason_invalid" || result.Problem.Owner != change.Owner || result.Problem.ResourceID != change.ResourceID || !result.Problem.RecoveryRequired {
			t.Fatalf("unsafe owner problem: %#v", result.Problem)
		}
	}
	if change.Problem != nil {
		t.Fatal("read projection mutated its input")
	}
}

func TestReadJobCannotInventCancellationDeadlineOrProgress(t *testing.T) {
	change := Change{Reference: NewReference("owner", "resource", "operation"), Revision: 9, ExpectedRevision: "owner-revision", OwnerPhase: "VERIFY", Outcome: OutcomePending}
	for _, progress := range []*Progress{nil, {Completed: 1, Total: 2, Unit: "percent"}, {Completed: -1, Total: 2, Unit: "bytes"}, {Completed: 3, Total: 2, Unit: "bytes"}, {Completed: 0, Total: 0, Unit: "bytes"}} {
		job := ProjectJob(change, progress, true)
		if job.Progress != nil || job.CancelSupported || job.CancelRequested != nil || job.Deadline != 0 || !job.CriticalSection || job.ResultRef != "" {
			t.Fatalf("projection invented execution authority/truth: %#v", job)
		}
	}
	progress := &Progress{Completed: 1, Total: 2, Unit: "bytes"}
	job := ProjectJob(change, progress, false)
	progress.Completed = 2
	if job.Progress == nil || job.Progress.Completed != 1 || job.Revision != 9 || job.ExpectedRevision != change.ExpectedRevision || job.Phase != change.OwnerPhase {
		t.Fatal("bounded read progress lost owner identity or aliased input")
	}
}

func TestTerminalJobRetainsOnlyRecordedOwnerOutcome(t *testing.T) {
	change := Change{Reference: NewReference("owner", "resource", "operation"), OwnerPhase: "ROLLBACK_FAILED", Outcome: OutcomeRecoveryRequired, RecoveryRequired: true}
	job := ProjectJob(change, nil, true)
	if job.Outcome != change.Outcome || !job.RecoveryRequired || job.ResultRef != change.OperationID || job.CancelSupported || job.CancelRequested != nil {
		t.Fatalf("terminal projection changed owner outcome: %#v", job)
	}
}
