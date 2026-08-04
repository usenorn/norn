package cycle

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const recordScopeChangeQuery = `
INSERT INTO workspace_cycle_scope_changes (
    id, cycle_id, issue_id, change, actor_account_id, changed_at
)
VALUES ($1, $2, $3, $4, $5, $6)`

const scopeChangesForCycleQuery = `
SELECT s.id,
       s.cycle_id,
       s.issue_id,
       s.change,
       coalesce(s.actor_account_id::text, ''),
       s.changed_at,
       i.reference_key || '-' || i.number::text,
       i.title
FROM workspace_cycle_scope_changes s
JOIN workspace_issues i ON i.id = s.issue_id
WHERE s.cycle_id = $1
ORDER BY s.changed_at, s.id`

type scopeChangeRepository struct {
	db *postgres.Client
}

func NewScopeChange(db *postgres.Client) repository.CycleScopeChange {
	return &scopeChangeRepository{db: db}
}

func (r *scopeChangeRepository) Record(ctx context.Context, change entity.CycleScopeChange) error {
	if change.ID == uuid.Nil {
		change.ID = uuid.New()
	}

	if change.ChangedAt.IsZero() {
		change.ChangedAt = time.Now().UTC()
	}

	var actor any

	if change.ActorAccountID != uuid.Nil {
		actor = change.ActorAccountID.String()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordScopeChangeQuery,
		change.ID.String(),
		change.CycleID.String(),
		change.IssueID.String(),
		string(change.Change),
		actor,
		change.ChangedAt,
	); err != nil {
		return fmt.Errorf("record cycle scope change: %w", err)
	}

	return nil
}

func (r *scopeChangeRepository) ListByCycleID(
	ctx context.Context,
	cycleID uuid.UUID,
) ([]entity.CycleScopeChange, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, scopeChangesForCycleQuery, cycleID.String())
	if err != nil {
		return nil, fmt.Errorf("list cycle scope changes: %w", err)
	}

	defer func() { _ = rows.Close() }()

	changes := make([]entity.CycleScopeChange, 0)

	for rows.Next() {
		var (
			change  entity.CycleScopeChange
			id      string
			cycle   string
			issue   string
			kind    string
			actorID string
		)

		if err := rows.Scan(
			&id,
			&cycle,
			&issue,
			&kind,
			&actorID,
			&change.ChangedAt,
			&change.IssueReference,
			&change.IssueTitle,
		); err != nil {
			return nil, fmt.Errorf("scan cycle scope change: %w", err)
		}

		change.Change = entity.CycleScopeChangeKind(kind)

		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("parse scope change id: %w", err)
		}

		change.ID = parsed

		if change.CycleID, err = uuid.Parse(cycle); err != nil {
			return nil, fmt.Errorf("parse scope change cycle id: %w", err)
		}

		if change.IssueID, err = uuid.Parse(issue); err != nil {
			return nil, fmt.Errorf("parse scope change issue id: %w", err)
		}

		if actorID != "" {
			if change.ActorAccountID, err = uuid.Parse(actorID); err != nil {
				return nil, fmt.Errorf("parse scope change actor id: %w", err)
			}
		}

		changes = append(changes, change)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cycle scope changes: %w", err)
	}

	return changes, nil
}
