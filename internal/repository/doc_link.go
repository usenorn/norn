package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=doc_link.go -destination=doclink/mock_doc_link.go -package=doclink -mock_names=DocLink=MockDocLink

type DocLink interface {
	Create(ctx context.Context, link entity.StoredDocLink) error
	Delete(ctx context.Context, workspaceID, docID, linkID uuid.UUID) error
	ListForDoc(ctx context.Context, workspaceID, docID uuid.UUID, scope entity.TeamScope) ([]entity.DocLink, error)
}
