package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=workflow_state.go -destination=workflowstate/mock_workflow_state.go -package=workflowstate -mock_names=WorkflowState=MockWorkflowState

type WorkflowState interface {
	Create(ctx context.Context, state entity.WorkflowState) (entity.WorkflowState, error)
	CreateMany(ctx context.Context, states []entity.WorkflowState) ([]entity.WorkflowState, error)
	ListByTeamID(ctx context.Context, teamID uuid.UUID) ([]entity.WorkflowState, error)
	LockByTeamID(ctx context.Context, teamID uuid.UUID) ([]entity.WorkflowState, error)
	DefaultForTeam(ctx context.Context, teamID uuid.UUID) (entity.WorkflowState, error)
	UpdateSettings(ctx context.Context, id uuid.UUID, name string, category entity.StateCategory) (entity.WorkflowState, error)
	Reposition(ctx context.Context, teamID uuid.UUID, orderedIDs []uuid.UUID) error
	SetDefault(ctx context.Context, teamID, stateID uuid.UUID) error
	SetCompletion(ctx context.Context, teamID, stateID uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}
