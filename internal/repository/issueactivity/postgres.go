package issueactivity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const recordActivityQuery = `
INSERT INTO workspace_issue_activity (
    id, workspace_id, issue_id, actor_account_id, kind,
    from_state_id, to_state_id, from_state_name, to_state_name,
    field, from_value, to_value, version, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

const activityPageQuery = `
SELECT a.id,
       a.workspace_id,
       a.issue_id,
       coalesce(a.actor_account_id::text, ''),
       coalesce(acct.display_name, ''),
       a.kind,
       a.from_state_name,
       a.to_state_name,
       coalesce(a.field, ''),
       coalesce(a.from_value, ''),
       coalesce(a.to_value, ''),
       coalesce(a.version, 0),
       a.created_at
FROM workspace_issue_activity a
LEFT JOIN accounts acct ON acct.id = a.actor_account_id
WHERE a.issue_id = $1
  AND ($2::boolean IS NOT TRUE
       OR (a.created_at, a.id) < ($3::timestamptz, $4::uuid))
ORDER BY a.created_at DESC, a.id DESC
LIMIT $5`

type issueActivityRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueActivity {
	return &issueActivityRepository{db: db}
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func optionalID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}

	return id.String()
}

func (r *issueActivityRepository) Record(ctx context.Context, activity entity.IssueActivity) error {
	if activity.ID == uuid.Nil {
		activity.ID = uuid.New()
	}

	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = time.Now().UTC()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordActivityQuery,
		activity.ID.String(),
		activity.WorkspaceID.String(),
		activity.IssueID.String(),
		optionalID(activity.ActorAccountID),
		string(activity.Kind),
		optionalID(activity.FromStateID),
		optionalID(activity.ToStateID),
		activity.FromState,
		activity.ToState,
		nullIfEmpty(activity.Field),
		nullIfEmpty(activity.FromValue),
		nullIfEmpty(activity.ToValue),
		activity.Version,
		activity.CreatedAt,
	); err != nil {
		return fmt.Errorf("record issue activity: %w", err)
	}

	return nil
}

func (r *issueActivityRepository) ListByIssueID(
	ctx context.Context,
	issueID uuid.UUID,
	page entity.IssueActivityPage,
) ([]entity.IssueActivity, error) {
	cursorCreatedAt := time.Time{}
	cursorID := uuid.Nil.String()

	if page.Cursor != nil {
		cursorCreatedAt = page.Cursor.CreatedAt
		cursorID = page.Cursor.ActivityID.String()
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		activityPageQuery,
		issueID.String(),
		page.Cursor != nil,
		cursorCreatedAt,
		cursorID,
		page.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list issue activity: %w", err)
	}

	defer func() { _ = rows.Close() }()

	entries := make([]entity.IssueActivity, 0, page.Limit)

	for rows.Next() {
		var (
			activity  entity.IssueActivity
			id        string
			workspace string
			issue     string
			actor     string
			kind      string
		)

		if err := rows.Scan(
			&id,
			&workspace,
			&issue,
			&actor,
			&activity.ActorName,
			&kind,
			&activity.FromState,
			&activity.ToState,
			&activity.Field,
			&activity.FromValue,
			&activity.ToValue,
			&activity.Version,
			&activity.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan issue activity: %w", err)
		}

		activity.Kind = entity.IssueActivityKind(kind)

		if activity.ID, err = uuid.Parse(id); err != nil {
			return nil, fmt.Errorf("parse issue activity id: %w", err)
		}

		if activity.WorkspaceID, err = uuid.Parse(workspace); err != nil {
			return nil, fmt.Errorf("parse issue activity workspace id: %w", err)
		}

		if activity.IssueID, err = uuid.Parse(issue); err != nil {
			return nil, fmt.Errorf("parse issue activity issue id: %w", err)
		}

		if actor != "" {
			if activity.ActorAccountID, err = uuid.Parse(actor); err != nil {
				return nil, fmt.Errorf("parse issue activity actor id: %w", err)
			}
		}

		entries = append(entries, activity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate issue activity: %w", err)
	}

	return entries, nil
}
