package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=issuerevision.go -destination=issuerevision/mock_issuerevision.go -package=issuerevision -mock_names=IssueRevision=MockIssueRevision

type IssueRevision interface {
	Record(ctx context.Context, revision entity.IssueDescriptionRevision) error
	List(ctx context.Context, workspaceID, issueID uuid.UUID, limit int) ([]entity.IssueDescriptionRevision, error)
	GetByID(ctx context.Context, workspaceID, revisionID uuid.UUID) (entity.IssueDescriptionRevision, error)
}
