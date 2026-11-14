// Package inboundcontrol declares typed inbound fallback facts and owner
// operations. The Core owner retains capability resolution, configuration,
// checkpoint materialization, storage admission and actual runtime execution.
// A snapshot or checkpoint identifier alone grants no mutation authority.
package inboundcontrol

import "context"

type Reader interface {
	Identity(context.Context) CoreRuntimeIdentityV1
	Snapshot(context.Context, uint) (InboundFallbackSnapshotV1, error)
	PreviewFallbackPatch(context.Context, PreviewFallbackPatchRequestV1) (FallbackPatchPreviewV1, error)
}

// Control is limited to an exact, revision-fenced fallback change and its
// corresponding checkpoint. The owner must validate every admission and fence.
type Control interface {
	Reader
	PrepareCheckpoint(context.Context, PrepareCheckpointRequestV1) (CheckpointPreparationV1, error)
	InspectCheckpoint(context.Context, InspectCheckpointRequestV1) (CheckpointStatusV1, error)
	FindCheckpoint(context.Context, FindCheckpointRequestV1) (CheckpointStatusV1, error)
	ApplyFallbackPatch(context.Context, ApplyFallbackPatchRequestV1) (FallbackMutationResultV1, error)
	VerifyEffective(context.Context, VerifyEffectiveRequestV1) (EffectiveVerificationV1, error)
	RestoreCheckpoint(context.Context, RestoreCheckpointRequestV1) (RestoreCheckpointResultV1, error)
	ReleaseCheckpoint(context.Context, ReleaseCheckpointRequestV1) (CheckpointReleaseV1, error)
}
