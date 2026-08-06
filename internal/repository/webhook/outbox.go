package webhook

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const recordOutboxQuery = `
INSERT INTO webhook_outbox (
    id, workspace_id, event, subject_kind, subject_id, team_id,
    actor_account_id, actor_kind, actor_name, body
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

const claimOutboxQuery = `
UPDATE webhook_outbox
SET fanned_out_at = now()
WHERE id IN (
    SELECT id
    FROM webhook_outbox
    WHERE fanned_out_at IS NULL
    ORDER BY occurred_at, id
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, workspace_id, event, subject_kind,
          coalesce(subject_id::text, ''), coalesce(team_id::text, ''),
          coalesce(actor_account_id::text, ''), actor_kind, actor_name,
          body, occurred_at`

type outboxRepository struct {
	db *postgres.Client
}

func NewOutbox(db *postgres.Client) repository.WebhookOutbox {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) Record(ctx context.Context, entry entity.WebhookOutboxEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordOutboxQuery,
		entry.ID.String(),
		entry.WorkspaceID.String(),
		string(entry.Event),
		entry.SubjectKind,
		optionalID(entry.SubjectID),
		optionalID(entry.TeamID),
		optionalID(entry.ActorID),
		string(entry.ActorKind),
		entry.ActorName,
		entry.Body,
	); err != nil {
		return fmt.Errorf("record webhook event: %w", err)
	}

	return nil
}

func (r *outboxRepository) ClaimPending(
	ctx context.Context,
	limit int,
) ([]entity.WebhookOutboxEntry, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, claimOutboxQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("claim webhook events: %w", err)
	}

	defer func() { _ = rows.Close() }()

	entries := make([]entity.WebhookOutboxEntry, 0)

	for rows.Next() {
		var (
			entry                         entity.WebhookOutboxEntry
			rawID, rawWorkspace, event    string
			rawSubject, rawTeam, rawActor string
			actorKind                     string
		)

		if err := rows.Scan(
			&rawID, &rawWorkspace, &event, &entry.SubjectKind,
			&rawSubject, &rawTeam, &rawActor, &actorKind, &entry.ActorName,
			&entry.Body, &entry.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan webhook event: %w", err)
		}

		entry.ID = parseID(rawID)
		entry.WorkspaceID = parseID(rawWorkspace)
		entry.Event = entity.WebhookEvent(event)
		entry.SubjectID = parseID(rawSubject)
		entry.TeamID = parseID(rawTeam)
		entry.ActorID = parseID(rawActor)
		entry.ActorKind = entity.ActorKind(actorKind)

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read webhook events: %w", err)
	}

	return entries, nil
}
