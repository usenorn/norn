package webhook

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const deliveryColumns = `
SELECT d.id, d.webhook_id, d.workspace_id,
       coalesce(d.outbox_id::text, ''), coalesce(d.replay_of::text, ''),
       d.event, d.subject_kind, coalesce(d.subject_id::text, ''),
       coalesce(d.team_id::text, ''), d.body, d.state, d.attempt,
       d.next_attempt_at, d.settled_at, d.created_at
FROM webhook_deliveries d`

const queueDeliveryQuery = `
INSERT INTO webhook_deliveries (
    id, webhook_id, workspace_id, outbox_id, replay_of,
    event, subject_kind, subject_id, team_id, body, next_attempt_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

const claimAttemptQuery = deliveryColumns + `
WHERE d.id = $1 AND d.state = 'pending' AND d.attempt = $2`

const recordAttemptQuery = `
INSERT INTO webhook_attempts (
    id, delivery_id, attempt, request_url, resolved_address,
    outcome, status_code, response_excerpt, error, started_at, finished_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

const attemptColumns = `
SELECT a.id, a.delivery_id, a.attempt, a.request_url,
       coalesce(host(a.resolved_address), ''), a.outcome,
       coalesce(a.status_code, 0), a.response_excerpt, a.error,
       a.started_at, a.finished_at
FROM webhook_attempts a`

type deliveryRepository struct {
	db *postgres.Client
}

func NewDeliveries(db *postgres.Client) repository.WebhookDelivery {
	return &deliveryRepository{db: db}
}

func (r *deliveryRepository) Queue(ctx context.Context, deliveries []entity.WebhookDelivery) error {
	executor := r.db.Querier(ctx)

	for _, delivery := range deliveries {
		if delivery.ID == uuid.Nil {
			delivery.ID = uuid.New()
		}

		if delivery.NextAttempt.IsZero() {
			delivery.NextAttempt = time.Now().UTC()
		}

		if _, err := executor.ExecContext(
			ctx,
			queueDeliveryQuery,
			delivery.ID.String(),
			delivery.WebhookID.String(),
			delivery.WorkspaceID.String(),
			optionalID(delivery.OutboxID),
			optionalID(delivery.ReplayOf),
			string(delivery.Event),
			delivery.SubjectKind,
			optionalID(delivery.SubjectID),
			optionalID(delivery.TeamID),
			delivery.Body,
			delivery.NextAttempt,
		); err != nil {
			return fmt.Errorf("queue webhook delivery: %w", err)
		}
	}

	return nil
}

func (r *deliveryRepository) Get(
	ctx context.Context,
	webhookID, deliveryID uuid.UUID,
) (entity.WebhookDelivery, error) {
	deliveries, err := r.many(
		ctx,
		deliveryColumns+` WHERE d.webhook_id = $1 AND d.id = $2`,
		webhookID.String(), deliveryID.String(),
	)
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	if len(deliveries) == 0 {
		return entity.WebhookDelivery{}, entity.ErrWebhookDeliveryNotFound
	}

	return deliveries[0], nil
}

func (r *deliveryRepository) List(
	ctx context.Context,
	filter entity.WebhookDeliveryFilter,
) ([]entity.WebhookDelivery, error) {
	where := make([]string, 0, 4)
	args := make([]any, 0, 6)

	bind := func(value any) string {
		args = append(args, value)

		return fmt.Sprintf("$%d", len(args))
	}

	where = append(where, "d.webhook_id = "+bind(filter.WebhookID.String()))

	if len(filter.Events) > 0 {
		placeholders := make([]string, 0, len(filter.Events))
		for _, event := range filter.Events {
			placeholders = append(placeholders, bind(string(event)))
		}

		where = append(where, "d.event IN ("+strings.Join(placeholders, ", ")+")")
	}

	if len(filter.States) > 0 {
		placeholders := make([]string, 0, len(filter.States))
		for _, state := range filter.States {
			placeholders = append(placeholders, bind(string(state)))
		}

		where = append(where, "d.state IN ("+strings.Join(placeholders, ", ")+")")
	}

	if filter.Cursor != nil {
		where = append(where, fmt.Sprintf(
			"(d.created_at, d.id) < (%s, %s::uuid)",
			bind(filter.Cursor.CreatedAt), bind(filter.Cursor.ID.String()),
		))
	}

	query := deliveryColumns + "\nWHERE " + strings.Join(where, "\n  AND ") +
		"\nORDER BY d.created_at DESC, d.id DESC\nLIMIT " + bind(filter.Limit)

	return r.many(ctx, query, args...)
}

func (r *deliveryRepository) ListDue(
	ctx context.Context,
	before time.Time,
	limit int,
) ([]entity.WebhookDelivery, error) {
	return r.many(
		ctx,
		deliveryColumns+`
		WHERE d.state = 'pending' AND d.next_attempt_at <= $1
		ORDER BY d.next_attempt_at, d.id
		LIMIT $2`,
		before, limit,
	)
}

func (r *deliveryRepository) Attempts(
	ctx context.Context,
	deliveryID uuid.UUID,
) ([]entity.WebhookAttempt, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		attemptColumns+` WHERE a.delivery_id = $1 ORDER BY a.attempt`,
		deliveryID.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("query webhook attempts: %w", err)
	}

	defer func() { _ = rows.Close() }()

	attempts := make([]entity.WebhookAttempt, 0)

	for rows.Next() {
		var (
			attempt            entity.WebhookAttempt
			rawID, rawDelivery string
			outcome            string
		)

		if err := rows.Scan(
			&rawID, &rawDelivery, &attempt.Attempt, &attempt.RequestURL,
			&attempt.ResolvedAddress, &outcome, &attempt.StatusCode,
			&attempt.ResponseExcerpt, &attempt.Error,
			&attempt.StartedAt, &attempt.FinishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan webhook attempt: %w", err)
		}

		attempt.ID = parseID(rawID)
		attempt.DeliveryID = parseID(rawDelivery)
		attempt.Outcome = entity.WebhookAttemptOutcome(outcome)

		attempts = append(attempts, attempt)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read webhook attempts: %w", err)
	}

	return attempts, nil
}

func (r *deliveryRepository) ClaimAttempt(
	ctx context.Context,
	deliveryID uuid.UUID,
	attempt int,
) (entity.WebhookDelivery, error) {
	claimed, err := r.many(ctx, claimAttemptQuery, deliveryID.String(), attempt)
	if err != nil {
		return entity.WebhookDelivery{}, err
	}

	if len(claimed) == 0 {
		return entity.WebhookDelivery{}, entity.ErrWebhookDeliveryNotFound
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE webhook_deliveries SET attempt = attempt + 1
		 WHERE id = $1 AND state = 'pending' AND attempt = $2`,
		deliveryID.String(), attempt,
	)
	if err != nil {
		return entity.WebhookDelivery{}, fmt.Errorf("claim webhook attempt: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return entity.WebhookDelivery{}, fmt.Errorf("claim webhook attempt: %w", err)
	}

	if affected == 0 {
		return entity.WebhookDelivery{}, entity.ErrWebhookDeliveryNotFound
	}

	delivery := claimed[0]
	delivery.Attempt = attempt + 1

	return delivery, nil
}

func (r *deliveryRepository) RecordAttempt(ctx context.Context, attempt entity.WebhookAttempt) error {
	if attempt.ID == uuid.Nil {
		attempt.ID = uuid.New()
	}

	var status any
	if attempt.StatusCode > 0 {
		status = attempt.StatusCode
	}

	var resolved any
	if attempt.ResolvedAddress != "" {
		resolved = attempt.ResolvedAddress
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		recordAttemptQuery,
		attempt.ID.String(),
		attempt.DeliveryID.String(),
		attempt.Attempt,
		attempt.RequestURL,
		resolved,
		string(attempt.Outcome),
		status,
		attempt.ResponseExcerpt,
		attempt.Error,
		attempt.StartedAt,
		attempt.FinishedAt,
	); err != nil {
		return fmt.Errorf("record webhook attempt: %w", err)
	}

	return nil
}

func (r *deliveryRepository) Reschedule(ctx context.Context, deliveryID uuid.UUID, at time.Time) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE webhook_deliveries SET next_attempt_at = $2 WHERE id = $1 AND state = 'pending'`,
		deliveryID.String(), at,
	); err != nil {
		return fmt.Errorf("reschedule webhook delivery: %w", err)
	}

	return nil
}

func (r *deliveryRepository) Settle(
	ctx context.Context,
	deliveryID uuid.UUID,
	state entity.WebhookDeliveryState,
	at time.Time,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE webhook_deliveries SET state = $2, settled_at = $3 WHERE id = $1`,
		deliveryID.String(), string(state), at,
	); err != nil {
		return fmt.Errorf("settle webhook delivery: %w", err)
	}

	return nil
}

func (r *deliveryRepository) many(
	ctx context.Context,
	query string,
	args ...any,
) ([]entity.WebhookDelivery, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query webhook deliveries: %w", err)
	}

	defer func() { _ = rows.Close() }()

	deliveries := make([]entity.WebhookDelivery, 0)

	for rows.Next() {
		var (
			delivery                         entity.WebhookDelivery
			rawID, rawWebhook, rawWorkspace  string
			rawOutbox, rawReplay, rawSubject string
			rawTeam, event, state            string
			settledAt                        sql.NullTime
		)

		if err := rows.Scan(
			&rawID, &rawWebhook, &rawWorkspace, &rawOutbox, &rawReplay,
			&event, &delivery.SubjectKind, &rawSubject, &rawTeam,
			&delivery.Body, &state, &delivery.Attempt,
			&delivery.NextAttempt, &settledAt, &delivery.CreatedAt,
		); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, entity.ErrWebhookDeliveryNotFound
			}

			return nil, fmt.Errorf("scan webhook delivery: %w", err)
		}

		delivery.ID = parseID(rawID)
		delivery.WebhookID = parseID(rawWebhook)
		delivery.WorkspaceID = parseID(rawWorkspace)
		delivery.OutboxID = parseID(rawOutbox)
		delivery.ReplayOf = parseID(rawReplay)
		delivery.SubjectID = parseID(rawSubject)
		delivery.TeamID = parseID(rawTeam)
		delivery.Event = entity.WebhookEvent(event)
		delivery.State = entity.WebhookDeliveryState(state)

		if settledAt.Valid {
			delivery.SettledAt = &settledAt.Time
		}

		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read webhook deliveries: %w", err)
	}

	return deliveries, nil
}
