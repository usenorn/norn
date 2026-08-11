package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=check.go -destination=check/mock_check.go -package=check -mock_names=Check=MockCheck,CheckEvidence=MockCheckEvidence

type CheckDecision struct {
	CheckID   uuid.UUID
	Approval  entity.CheckApproval
	AccountID uuid.UUID
	DecidedAt time.Time
}

type CheckResolutionInput struct {
	CheckID    uuid.UUID
	Resolution entity.CheckResolution
	Reason     string
	GapIssueID uuid.UUID
	AccountID  uuid.UUID
	ResolvedAt time.Time
}

type StaleIssue struct {
	WorkspaceID uuid.UUID
	IssueID     uuid.UUID
}

type Check interface {
	Create(ctx context.Context, check entity.Check) (entity.Check, error)
	GetByID(ctx context.Context, workspaceID, checkID uuid.UUID) (entity.Check, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.Check, error)
	Decide(ctx context.Context, workspaceID uuid.UUID, decision CheckDecision) (entity.Check, error)
	Resolve(ctx context.Context, workspaceID uuid.UUID, resolution CheckResolutionInput) (entity.Check, error)
	Delete(ctx context.Context, workspaceID, issueID, checkID uuid.UUID) error
	ListStaleIssues(ctx context.Context, window time.Duration, limit int) ([]StaleIssue, error)
	AnnounceExpiry(ctx context.Context, workspaceID, checkID, evidenceID uuid.UUID) error
}

type CheckEvidence interface {
	Append(ctx context.Context, evidence entity.Evidence) (entity.Evidence, error)
	ListByCheck(ctx context.Context, workspaceID, checkID uuid.UUID) ([]entity.Evidence, error)
	ListByIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.Evidence, error)
	Digest(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.Evidence, error)
}
