package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=doc_passage.go -destination=docpassage/mock_doc_passage.go -package=docpassage -mock_names=DocPassage=MockDocPassage

type DocPassage interface {
	Replace(ctx context.Context, doc entity.Doc) error
	Outdated(ctx context.Context, workspaceID uuid.UUID, limit int) ([]uuid.UUID, error)
}
