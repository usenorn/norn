package executionupload

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

func (s *executionUploadsService) Policy(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.WorkspaceExecutionPolicy, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionRead,
		WorkspaceID: workspaceID,
	}); err != nil {
		return entity.WorkspaceExecutionPolicy{}, err
	}

	return s.effective(ctx, workspaceID)
}

func (s *executionUploadsService) SetPolicy(
	ctx context.Context,
	policy entity.WorkspaceExecutionPolicy,
) (entity.WorkspaceExecutionPolicy, error) {
	if _, err := s.authorizer.Decide(ctx, entity.AccessRequest{
		Resource:    entity.ResourceWorkspace,
		Action:      entity.ActionUpdate,
		WorkspaceID: policy.WorkspaceID,
	}); err != nil {
		return entity.WorkspaceExecutionPolicy{}, err
	}

	if err := entity.NewValidationError(
		entity.ValidateTelemetryMode("telemetry", policy.Telemetry),
		entity.ValidateUploadRetentionDays("uploadRetentionDays", policy.UploadRetentionDays),
	); err != nil {
		return entity.WorkspaceExecutionPolicy{}, err
	}

	return s.policies.Upsert(ctx, policy)
}
