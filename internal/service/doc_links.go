package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=doc_links.go -destination=doclink/mock_doc_links.go -package=doclink -mock_names=DocLinks=MockDocLinks

type LinkDocInput struct {
	IssueID   uuid.UUID
	ProjectID uuid.UUID
}

type DocLinks interface {
	Link(ctx context.Context, workspaceID, docID uuid.UUID, input LinkDocInput) (entity.DocLink, error)
	Unlink(ctx context.Context, workspaceID, docID, linkID uuid.UUID) error
	List(ctx context.Context, workspaceID, docID uuid.UUID) ([]entity.DocLink, error)
	ForIssue(ctx context.Context, workspaceID, issueID uuid.UUID) ([]entity.DocSummary, error)
	ForProject(ctx context.Context, workspaceID, projectID uuid.UUID) ([]entity.DocSummary, error)
}
