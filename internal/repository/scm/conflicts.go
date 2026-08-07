package scm

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type conflictRepository struct {
	db *postgres.Client
}

func NewMirrorConflict(db *postgres.Client) repository.MirrorConflict {
	return &conflictRepository{db: db}
}

const recordConflictQuery = `
INSERT INTO workspace_issue_mirror_conflicts (
    id, workspace_id, mirror_id, issue_id, field, winner, discarded, kept, occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

func (r *conflictRepository) Record(
	ctx context.Context,
	conflict entity.MirrorConflict,
) error {
	if conflict.ID == uuid.Nil {
		conflict.ID = uuid.New()
	}

	_, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordConflictQuery,
		conflict.ID,
		conflict.WorkspaceID,
		conflict.MirrorID,
		conflict.IssueID,
		conflict.Field,
		conflict.Winner,
		conflict.Discarded,
		conflict.Kept,
		conflict.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("record a discarded edit: %w", err)
	}

	return nil
}

const listConflictsQuery = `
SELECT id, workspace_id, mirror_id, issue_id, field, winner, discarded, kept, occurred_at
FROM workspace_issue_mirror_conflicts
WHERE workspace_id = $1 AND issue_id = $2
ORDER BY occurred_at DESC, id
LIMIT $3`

func (r *conflictRepository) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
	limit int,
) ([]entity.MirrorConflict, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listConflictsQuery, workspaceID, issueID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list discarded edits: %w", err)
	}

	defer func() { _ = rows.Close() }()

	conflicts := make([]entity.MirrorConflict, 0)

	for rows.Next() {
		var conflict entity.MirrorConflict

		if err := rows.Scan(
			&conflict.ID,
			&conflict.WorkspaceID,
			&conflict.MirrorID,
			&conflict.IssueID,
			&conflict.Field,
			&conflict.Winner,
			&conflict.Discarded,
			&conflict.Kept,
			&conflict.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("read a discarded edit: %w", err)
		}

		conflicts = append(conflicts, conflict)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list discarded edits: %w", err)
	}

	return conflicts, nil
}
