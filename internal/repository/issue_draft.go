package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issue_draft.go -destination=issuedraft/mock_issue_draft.go -package=issuedraft -mock_names=IssueDraft=MockIssueDraft

type IssueDraft interface {
	Save(ctx context.Context, draft entity.IssueDraft) (entity.IssueDraft, error)
	List(ctx context.Context, workspaceID, accountID uuid.UUID, limit int) ([]entity.IssueDraft, error)
	GetByID(ctx context.Context, workspaceID, accountID, draftID uuid.UUID) (entity.IssueDraft, error)
	Count(ctx context.Context, workspaceID, accountID uuid.UUID) (int, error)
	Remove(ctx context.Context, workspaceID, accountID, draftID uuid.UUID) error
}
