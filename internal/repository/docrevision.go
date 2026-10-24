package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=docrevision.go -destination=docrevision/mock_docrevision.go -package=docrevision -mock_names=DocRevision=MockDocRevision

type DocRevision interface {
	Record(ctx context.Context, revision entity.DocRevision) error
	List(
		ctx context.Context,
		workspaceID, docID uuid.UUID,
		page entity.DocRevisionPage,
	) ([]entity.DocRevisionSummary, error)
	GetByID(ctx context.Context, workspaceID, docID, revisionID uuid.UUID) (entity.DocRevision, error)
}
