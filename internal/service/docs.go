package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=docs.go -destination=doc/mock_docs.go -package=doc -mock_names=Docs=MockDocs

type CreateDocInput struct {
	Title    string
	Markdown string
	Content  *entity.Document
	ParentID *uuid.UUID
	Audience entity.KnowledgeAudience
}

type UpdateDocInput struct {
	ExpectedVersion int
	Title           *string
	Markdown        *string
	Content         *entity.Document
}

type MoveDocInput struct {
	ExpectedVersion int
	ParentID        *uuid.UUID
}

type ReshareDocInput struct {
	ExpectedVersion int
	Audience        entity.KnowledgeAudience
}

type ListDocsInput struct {
	ParentID *uuid.UUID
	Root     bool
	Cursor   string
	Limit    int
}

type DocList struct {
	Docs       []entity.DocSummary
	NextCursor string
}

type ListDocRevisionsInput struct {
	Cursor string
	Limit  int
}

type DocHistory struct {
	Revisions  []entity.DocRevisionSummary
	NextCursor string
}

type Docs interface {
	Create(ctx context.Context, workspaceID uuid.UUID, input CreateDocInput) (entity.Doc, error)
	Get(ctx context.Context, workspaceID, docID uuid.UUID) (entity.Doc, error)
	List(ctx context.Context, workspaceID uuid.UUID, input ListDocsInput) (DocList, error)
	Update(ctx context.Context, workspaceID, docID uuid.UUID, input UpdateDocInput) (entity.Doc, error)
	Move(ctx context.Context, workspaceID, docID uuid.UUID, input MoveDocInput) (entity.Doc, error)
	Reshare(ctx context.Context, workspaceID, docID uuid.UUID, input ReshareDocInput) (entity.Doc, error)
	Archive(ctx context.Context, workspaceID, docID uuid.UUID, expectedVersion int) (entity.Doc, error)
	Revisions(
		ctx context.Context,
		workspaceID, docID uuid.UUID,
		input ListDocRevisionsInput,
	) (DocHistory, error)
	Revision(ctx context.Context, workspaceID, docID, revisionID uuid.UUID) (entity.DocRevision, error)
	Restore(
		ctx context.Context,
		workspaceID, docID, revisionID uuid.UUID,
		expectedVersion int,
	) (entity.Doc, error)
}
