package docpassage

import (
	"context"
	"fmt"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const outdatedDocsQuery = `
SELECT d.id
FROM workspace_docs d
WHERE d.workspace_id = $1
  AND NOT EXISTS (
      SELECT 1 FROM workspace_doc_passages p
      WHERE p.doc_id = d.id AND p.doc_version = d.version)
ORDER BY d.id
LIMIT $2`

type docPassageRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.DocPassage {
	return &docPassageRepository{db: db}
}

func toModel(doc entity.Doc, passage entity.DocPassage) *dbpostgres.WorkspaceDocPassage {
	return &dbpostgres.WorkspaceDocPassage{
		WorkspaceID: doc.WorkspaceID.String(),
		DocID:       doc.ID.String(),
		DocVersion:  doc.Version,
		Position:    passage.Position,
		Heading:     passage.Heading,
		Body:        passage.Body,
	}
}

func (r *docPassageRepository) Replace(ctx context.Context, doc entity.Doc) error {
	executor := r.db.Querier(ctx)

	if _, err := dbpostgres.WorkspaceDocPassages(
		dbpostgres.WorkspaceDocPassageWhere.WorkspaceID.EQ(doc.WorkspaceID.String()),
		dbpostgres.WorkspaceDocPassageWhere.DocID.EQ(doc.ID.String()),
	).DeleteAll(ctx, executor); err != nil {
		return fmt.Errorf("clear doc passages: %w", err)
	}

	for _, passage := range doc.Passages() {
		if err := toModel(doc, passage).Insert(ctx, executor, boil.Infer()); err != nil {
			return fmt.Errorf("insert doc passage: %w", err)
		}
	}

	return nil
}

func (r *docPassageRepository) Outdated(
	ctx context.Context,
	workspaceID uuid.UUID,
	limit int,
) ([]uuid.UUID, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, outdatedDocsQuery, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list docs with outdated passages: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var docIDs []uuid.UUID

	for rows.Next() {
		var docID uuid.UUID
		if err := rows.Scan(&docID); err != nil {
			return nil, fmt.Errorf("scan doc with outdated passages: %w", err)
		}

		docIDs = append(docIDs, docID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read docs with outdated passages: %w", err)
	}

	return docIDs, nil
}
