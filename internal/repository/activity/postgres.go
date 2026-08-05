package activity

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
INSERT INTO workspace_activity (
    id, operation_id, workspace_id, issue_id, project_id,
    actor_account_id, actor_kind, actor_token_id, actor_token_name,
    actor_connection_id, actor_connection_name, kind,
    from_state_name, to_state_name,
    field, from_value, to_value, version, bulk_action_id, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`

const activityColumns = `
SELECT a.id,
       a.operation_id,
       a.workspace_id,
       coalesce(a.issue_id::text, ''),
       coalesce(a.project_id::text, ''),
       coalesce(a.actor_account_id::text, ''),
       coalesce(acct.display_name, ''),
       a.actor_kind,
       coalesce(a.actor_token_id::text, ''),
       coalesce(a.actor_token_name, ''),
       coalesce(a.actor_connection_id::text, ''),
       coalesce(a.actor_connection_name, ''),
       a.kind,
       a.from_state_name,
       a.to_state_name,
       coalesce(a.field, ''),
       coalesce(a.from_value, ''),
       coalesce(a.to_value, ''),
       coalesce(a.version, 0),
       coalesce(a.bulk_action_id::text, ''),
       a.created_at
FROM workspace_activity a
LEFT JOIN accounts acct ON acct.id = a.actor_account_id`

const issueSubject = `a.issue_id = $1`

const projectSubject = `a.project_id = $1`

const oldestLeaders = `
WHERE %s
  AND a.id = a.operation_id
  AND ($2::boolean IS NOT TRUE
       OR (a.created_at, a.id) > ($3::timestamptz, $4::uuid))
ORDER BY a.created_at, a.id
LIMIT $5`

const newestLeaders = `
WHERE %s
  AND a.id = a.operation_id
  AND ($2::boolean IS NOT TRUE
       OR (a.created_at, a.id) < ($3::timestamptz, $4::uuid))
ORDER BY a.created_at DESC, a.id DESC
LIMIT $5`

const operationRows = `
WHERE %s
  AND a.operation_id = ANY($2::uuid[])
ORDER BY a.created_at, a.id`

const actorLeadersOldest = `
WHERE a.workspace_id = $1
  AND a.actor_account_id = $2
  AND a.id = a.operation_id
  AND ($3::boolean IS NOT TRUE
       OR (a.created_at, a.id) > ($4::timestamptz, $5::uuid))
ORDER BY a.created_at, a.id
LIMIT $6`

const actorLeadersNewest = `
WHERE a.workspace_id = $1
  AND a.actor_account_id = $2
  AND a.id = a.operation_id
  AND ($3::boolean IS NOT TRUE
       OR (a.created_at, a.id) < ($4::timestamptz, $5::uuid))
ORDER BY a.created_at DESC, a.id DESC
LIMIT $6`

const actorOperationRows = `
WHERE a.workspace_id = $1
  AND a.operation_id = ANY($2::uuid[])
ORDER BY a.created_at, a.id`

type activityRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Activity {
	return &activityRepository{db: db}
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

func subjectPredicate(kind entity.ActivitySubjectKind) string {
	if kind == entity.ActivitySubjectProject {
		return projectSubject
	}

	return issueSubject
}

func (r *activityRepository) Record(ctx context.Context, activity entity.Activity) error {
	if activity.ID == uuid.Nil {
		activity.ID = uuid.New()
	}

	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = time.Now().UTC()
	}

	if activity.OperationID == uuid.Nil {
		activity.OperationID = postgres.Operation(ctx, activity.Subject.ID, activity.ID)
	}

	if activity.Actor.Kind == "" {
		activity.Actor.Kind = entity.ActorKindUser
	}

	issueID, projectID := activity.Subject.ID, uuid.Nil
	if activity.Subject.Kind == entity.ActivitySubjectProject {
		issueID, projectID = uuid.Nil, activity.Subject.ID
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordActivityQuery,
		activity.ID.String(),
		activity.OperationID.String(),
		activity.WorkspaceID.String(),
		optionalID(issueID),
		optionalID(projectID),
		optionalID(activity.Actor.AccountID),
		string(activity.Actor.Kind),
		optionalTokenID(activity.Actor.TokenID),
		nullIfEmpty(activity.Actor.TokenName),
		optionalTokenID(activity.Actor.ConnectionID),
		nullIfEmpty(activity.Actor.ConnectionName),
		string(activity.Kind),
		activity.FromState,
		activity.ToState,
		nullIfEmpty(activity.Field),
		nullIfEmpty(activity.FromValue),
		nullIfEmpty(activity.ToValue),
		activity.Version,
		optionalID(activity.BulkActionID),
		activity.CreatedAt,
	); err != nil {
		return fmt.Errorf("record activity: %w", err)
	}

	return nil
}

func (r *activityRepository) ListBySubject(
	ctx context.Context,
	subject entity.ActivitySubject,
	page entity.ActivityPage,
) ([]entity.ActivityEvent, error) {
	leaders, err := r.leaders(ctx, subject, page)
	if err != nil {
		return nil, err
	}

	if len(leaders) == 0 {
		return []entity.ActivityEvent{}, nil
	}

	operations := make([]string, 0, len(leaders))
	for _, leader := range leaders {
		operations = append(operations, leader.OperationID.String())
	}

	rows, err := r.query(
		ctx,
		activityColumns+fmt.Sprintf(operationRows, subjectPredicate(subject.Kind)),
		subject.ID.String(), operations,
	)
	if err != nil {
		return nil, err
	}

	return assemble(leaders, rows), nil
}

func assemble(leaders, rows []entity.Activity) []entity.ActivityEvent {
	events := make([]entity.ActivityEvent, 0, len(leaders))
	at := make(map[uuid.UUID]int, len(leaders))

	for _, leader := range leaders {
		at[leader.OperationID] = len(events)
		events = append(events, entity.NewActivityEvent(leader))
	}

	for _, row := range rows {
		index, ok := at[row.OperationID]
		if !ok {
			continue
		}

		events[index].Changes = append(events[index].Changes, row)
	}

	return events
}

func (r *activityRepository) ListByActor(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	page entity.ActivityPage,
) ([]entity.ActivityEvent, error) {
	window := actorLeadersOldest
	if page.Order == entity.ActivityOrderNewest {
		window = actorLeadersNewest
	}

	cursorCreatedAt := time.Time{}
	cursorID := uuid.Nil.String()

	if page.Cursor != nil {
		cursorCreatedAt = page.Cursor.CreatedAt
		cursorID = page.Cursor.OperationID.String()
	}

	leaders, err := r.query(
		ctx,
		activityColumns+window,
		workspaceID.String(), accountID.String(),
		page.Cursor != nil, cursorCreatedAt, cursorID, page.Limit,
	)
	if err != nil {
		return nil, err
	}

	if len(leaders) == 0 {
		return []entity.ActivityEvent{}, nil
	}

	operations := make([]string, 0, len(leaders))
	for _, leader := range leaders {
		operations = append(operations, leader.OperationID.String())
	}

	rows, err := r.query(
		ctx,
		activityColumns+actorOperationRows,
		workspaceID.String(), operations,
	)
	if err != nil {
		return nil, err
	}

	return assemble(leaders, rows), nil
}

func (r *activityRepository) leaders(
	ctx context.Context,
	subject entity.ActivitySubject,
	page entity.ActivityPage,
) ([]entity.Activity, error) {
	window := oldestLeaders
	if page.Order == entity.ActivityOrderNewest {
		window = newestLeaders
	}

	cursorCreatedAt := time.Time{}
	cursorID := uuid.Nil.String()

	if page.Cursor != nil {
		cursorCreatedAt = page.Cursor.CreatedAt
		cursorID = page.Cursor.OperationID.String()
	}

	return r.query(
		ctx,
		activityColumns+fmt.Sprintf(window, subjectPredicate(subject.Kind)),
		subject.ID.String(), page.Cursor != nil, cursorCreatedAt, cursorID, page.Limit,
	)
}

func (r *activityRepository) query(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.Activity, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read activity: %w", err)
	}

	defer func() { _ = rows.Close() }()

	entries := make([]entity.Activity, 0)

	for rows.Next() {
		var (
			activity         entity.Activity
			id, operation    string
			workspace        string
			issue, project   string
			actor, actorKind string
			actorToken       string
			actorConnection  string
			kind, bulkAction string
		)

		if err := rows.Scan(
			&id, &operation, &workspace, &issue, &project,
			&actor, &activity.ActorName, &actorKind, &actorToken, &activity.Actor.TokenName,
			&actorConnection, &activity.Actor.ConnectionName, &kind,
			&activity.FromState, &activity.ToState,
			&activity.Field, &activity.FromValue, &activity.ToValue,
			&activity.Version, &bulkAction, &activity.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}

		activity.Kind = entity.ActivityKind(kind)
		activity.Actor.Kind = entity.ActorKind(actorKind)

		if actorToken != "" {
			tokenID, err := uuid.Parse(actorToken)
			if err != nil {
				return nil, fmt.Errorf("parse activity actor token id: %w", err)
			}

			activity.Actor.TokenID = &tokenID
		}

		if actorConnection != "" {
			connectionID, err := uuid.Parse(actorConnection)
			if err != nil {
				return nil, fmt.Errorf("parse activity actor connection id: %w", err)
			}

			activity.Actor.ConnectionID = &connectionID
		}

		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("parse activity id: %w", err)
		}

		activity.ID = parsed

		if activity.OperationID, err = uuid.Parse(operation); err != nil {
			return nil, fmt.Errorf("parse activity operation id: %w", err)
		}

		if activity.WorkspaceID, err = uuid.Parse(workspace); err != nil {
			return nil, fmt.Errorf("parse activity workspace id: %w", err)
		}

		if issue != "" {
			subjectID, err := uuid.Parse(issue)
			if err != nil {
				return nil, fmt.Errorf("parse activity issue id: %w", err)
			}

			activity.Subject = entity.IssueSubject(subjectID)
		}

		if project != "" {
			subjectID, err := uuid.Parse(project)
			if err != nil {
				return nil, fmt.Errorf("parse activity project id: %w", err)
			}

			activity.Subject = entity.ProjectSubject(subjectID)
		}

		if actor != "" {
			if activity.Actor.AccountID, err = uuid.Parse(actor); err != nil {
				return nil, fmt.Errorf("parse activity actor id: %w", err)
			}
		}

		if bulkAction != "" {
			if activity.BulkActionID, err = uuid.Parse(bulkAction); err != nil {
				return nil, fmt.Errorf("parse activity bulk action id: %w", err)
			}
		}

		entries = append(entries, activity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity: %w", err)
	}

	return entries, nil
}

func optionalTokenID(tokenID *uuid.UUID) any {
	if tokenID == nil {
		return nil
	}

	return tokenID.String()
}
