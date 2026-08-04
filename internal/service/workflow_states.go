package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workflow_states.go -destination=workflowstate/mock_workflow_states.go -package=workflowstate -mock_names=WorkflowStates=MockWorkflowStates

type WorkflowStates interface {
	List(ctx context.Context, workspaceID, teamID uuid.UUID) ([]entity.WorkflowState, error)
	Create(ctx context.Context, input CreateWorkflowStateInput) (entity.WorkflowState, error)
	Update(ctx context.Context, workspaceID, teamID, stateID uuid.UUID, input UpdateWorkflowStateInput) (entity.WorkflowState, error)
	Reorder(ctx context.Context, workspaceID, teamID uuid.UUID, orderedStateIDs []uuid.UUID) ([]entity.WorkflowState, error)
	SetDefault(ctx context.Context, workspaceID, teamID, stateID uuid.UUID) ([]entity.WorkflowState, error)
	SetCompletion(ctx context.Context, workspaceID, teamID, stateID uuid.UUID) ([]entity.WorkflowState, error)
	Remove(ctx context.Context, workspaceID, teamID, stateID, replacementStateID uuid.UUID) error
}
