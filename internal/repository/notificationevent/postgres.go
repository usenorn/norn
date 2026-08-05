package notificationevent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const recordEventQuery = `
INSERT INTO workspace_notification_events (
    id, workspace_id, subject_kind, subject_id,
    issue_id, project_id, team_id,
    kind, actor_account_id, actor_kind, target_account_id,
    comment_id, bulk_action_id, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

const claimPendingQuery = `
UPDATE workspace_notification_events
SET fanned_out_at = now()
WHERE id IN (
    SELECT id
    FROM workspace_notification_events
    WHERE fanned_out_at IS NULL
    ORDER BY created_at, id
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, workspace_id, subject_kind, subject_id,
          kind, coalesce(actor_account_id::text, ''), actor_kind,
          coalesce(target_account_id::text, ''), coalesce(comment_id::text, ''),
          coalesce(bulk_action_id::text, ''), created_at`

type eventRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.NotificationEvent {
	return &eventRepository{db: db}
}

func optionalID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}

	return id.String()
}

func parse(raw string) uuid.UUID {
	if raw == "" {
		return uuid.Nil
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}

	return parsed
}

func (r *eventRepository) Record(ctx context.Context, event entity.NotificationEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	if event.ActorKind == "" {
		event.ActorKind = entity.ActorKindUser
	}

	issueID, projectID, teamID := uuid.Nil, uuid.Nil, uuid.Nil

	switch event.Subject.Kind {
	case entity.NotificationSubjectProject:
		projectID = event.Subject.ID
	case entity.NotificationSubjectTeam:
		teamID = event.Subject.ID
	default:
		issueID = event.Subject.ID
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordEventQuery,
		event.ID.String(),
		event.WorkspaceID.String(),
		string(event.Subject.Kind),
		event.Subject.ID.String(),
		optionalID(issueID),
		optionalID(projectID),
		optionalID(teamID),
		string(event.Kind),
		optionalID(event.Actor),
		string(event.ActorKind),
		optionalID(event.Target),
		optionalID(event.CommentID),
		optionalID(event.BulkActionID),
		event.CreatedAt,
	); err != nil {
		return fmt.Errorf("record notification event: %w", err)
	}

	return nil
}

func (r *eventRepository) ClaimPending(ctx context.Context, limit int) ([]entity.NotificationEvent, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, claimPendingQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("claim notification events: %w", err)
	}

	defer func() { _ = rows.Close() }()

	events := make([]entity.NotificationEvent, 0, limit)

	for rows.Next() {
		var (
			event                        entity.NotificationEvent
			id, workspace, subject       string
			subjectKind, kind, actorKind string
			actor, target                string
			comment, bulkAction          string
		)

		if err := rows.Scan(
			&id, &workspace, &subjectKind, &subject,
			&kind, &actor, &actorKind,
			&target, &comment, &bulkAction, &event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification event: %w", err)
		}

		event.ID = parse(id)
		event.WorkspaceID = parse(workspace)
		event.Subject = entity.NotificationSubject{
			Kind: entity.NotificationSubjectKind(subjectKind),
			ID:   parse(subject),
		}
		event.Kind = entity.NotificationKind(kind)
		event.Actor = parse(actor)
		event.ActorKind = entity.ActorKind(actorKind)
		event.Target = parse(target)
		event.CommentID = parse(comment)
		event.BulkActionID = parse(bulkAction)

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read notification events: %w", err)
	}

	return events, nil
}
