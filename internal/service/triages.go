package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=triages.go -destination=triage/mock_triages.go -package=triage -mock_names=Triages=MockTriages

type TriageQueueInput struct {
	Limit  int
	Cursor string
}

type TriageQueue struct {
	Issues     []entity.Issue
	NextCursor string
	Teams      []entity.IssueGroupTally
}

type ConfigureTriageInput struct {
	RouteAgents       bool
	RouteIntegrations bool
	RouteNonMembers   bool
}

type ReassignTriageInput struct {
	TeamID               uuid.UUID
	ExpectedVersion      int
	AcknowledgeLabelLoss bool
}

type Triages interface {
	Queue(ctx context.Context, workspaceID uuid.UUID, input TriageQueueInput) (TriageQueue, error)
	Accept(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.Issue, error)
	Decline(ctx context.Context, workspaceID, issueID uuid.UUID) (entity.Issue, error)
	Merge(ctx context.Context, workspaceID, issueID, duplicateOfID uuid.UUID) (entity.Issue, error)
	Reassign(ctx context.Context, workspaceID, issueID uuid.UUID, input ReassignTriageInput) (entity.Issue, error)
	Settings(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.TriageSettings, error)
	Configure(ctx context.Context, workspaceID, teamID uuid.UUID, input ConfigureTriageInput) (entity.TriageSettings, error)
	Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error
}
