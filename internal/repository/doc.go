package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=doc.go -destination=doc/mock_doc.go -package=doc -mock_names=Doc=MockDoc

type DocFilter struct {
	ParentID         *uuid.UUID
	Root             bool
	AllProjects      bool
	ProjectIDs       []uuid.UUID
	PersonalAuthorID uuid.UUID
}

type Doc interface {
	Create(ctx context.Context, doc entity.Doc) error
	GetByID(ctx context.Context, workspaceID, docID uuid.UUID) (entity.Doc, error)
	LockByID(ctx context.Context, workspaceID, docID uuid.UUID) (entity.Doc, error)
	Update(ctx context.Context, doc entity.Doc, expectedVersion int) error
	List(ctx context.Context, workspaceID uuid.UUID, filter DocFilter, page entity.DocPage) ([]entity.DocSummary, error)
	Ancestry(ctx context.Context, workspaceID, docID uuid.UUID) ([]uuid.UUID, error)
	SubtreeHeight(ctx context.Context, workspaceID, docID uuid.UUID) (int, error)
	HasActiveChildren(ctx context.Context, workspaceID, docID uuid.UUID) (bool, error)
}
