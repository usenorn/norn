package scm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type deliveryRepository struct {
	db *postgres.Client
}

func NewSCMDelivery(db *postgres.Client) repository.SCMDelivery {
	return &deliveryRepository{db: db}
}

const deliveryColumns = `
    id, repository_id, workspace_id, external_delivery_id, event, payload,
    attempt, retry_after, outcome, detail, failure, received_at, processed_at`

func scanDelivery(row interface{ Scan(...any) error }) (entity.SCMDelivery, error) {
	var delivery entity.SCMDelivery

	err := row.Scan(
		&delivery.ID,
		&delivery.RepositoryID,
		&delivery.WorkspaceID,
		&delivery.ExternalID,
		&delivery.Event,
		&delivery.Payload,
		&delivery.Attempt,
		&delivery.RetryAfter,
		&delivery.Outcome,
		&delivery.Detail,
		&delivery.Failure,
		&delivery.ReceivedAt,
		&delivery.ProcessedAt,
	)
	if err != nil {
		return entity.SCMDelivery{}, err
	}

	return delivery, nil
}

const recordDeliveryQuery = `
INSERT INTO workspace_scm_deliveries (
    id, repository_id, workspace_id, external_delivery_id, event, payload
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id`

func (r *deliveryRepository) Record(
	ctx context.Context,
	delivery entity.SCMDelivery,
) (uuid.UUID, error) {
	if delivery.ID == uuid.Nil {
		delivery.ID = uuid.New()
	}

	var id uuid.UUID

	err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		recordDeliveryQuery,
		delivery.ID,
		delivery.RepositoryID,
		delivery.WorkspaceID,
		delivery.ExternalID,
		delivery.Event,
		delivery.Payload,
	).Scan(&id)
	if err != nil {
		if violates(err, deliveryUniqueIndex) {
			return uuid.Nil, entity.ErrSCMDeliveryDuplicate
		}

		return uuid.Nil, fmt.Errorf("record source control delivery: %w", err)
	}

	return id, nil
}

const getDeliveryQuery = `
SELECT` + deliveryColumns + `
FROM workspace_scm_deliveries
WHERE id = $1`

func (r *deliveryRepository) GetByID(
	ctx context.Context,
	deliveryID uuid.UUID,
) (entity.SCMDelivery, error) {
	delivery, err := scanDelivery(
		r.db.Querier(ctx).QueryRowContext(ctx, getDeliveryQuery, deliveryID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMDelivery{}, entity.ErrSCMConnectionNotFound
	}

	if err != nil {
		return entity.SCMDelivery{}, fmt.Errorf("read source control delivery: %w", err)
	}

	return delivery, nil
}

const settleDeliveryQuery = `
UPDATE workspace_scm_deliveries
SET processed_at = $2, outcome = $3, detail = $4, failure = $5, retry_after = NULL
WHERE id = $1`

func (r *deliveryRepository) Settle(
	ctx context.Context,
	deliveryID uuid.UUID,
	outcome entity.SCMDeliveryOutcome,
	detail string,
	at time.Time,
) error {
	failure := ""
	if outcome == entity.SCMDeliveryFailed {
		failure = detail
	}

	result, err := r.db.Querier(ctx).
		ExecContext(ctx, settleDeliveryQuery, deliveryID, at, outcome, detail, failure)
	if err != nil {
		return fmt.Errorf("settle source control delivery: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const rescheduleDeliveryQuery = `
UPDATE workspace_scm_deliveries
SET attempt = $2, retry_after = $3, failure = $4
WHERE id = $1`

func (r *deliveryRepository) Reschedule(
	ctx context.Context,
	deliveryID uuid.UUID,
	attempt int,
	retryAfter time.Time,
	failure string,
) error {
	result, err := r.db.Querier(ctx).
		ExecContext(ctx, rescheduleDeliveryQuery, deliveryID, attempt, retryAfter, failure)
	if err != nil {
		return fmt.Errorf("reschedule source control delivery: %w", err)
	}

	return expectOne(result, entity.ErrSCMConnectionNotFound)
}

const listPendingDeliveriesQuery = `
SELECT` + deliveryColumns + `
FROM workspace_scm_deliveries
WHERE repository_id = $1
  AND processed_at IS NULL
  AND (retry_after IS NULL OR retry_after <= $2)
ORDER BY received_at, id
LIMIT $3`

func (r *deliveryRepository) ListPending(
	ctx context.Context,
	repositoryID uuid.UUID,
	at time.Time,
	limit int,
) ([]entity.SCMDelivery, error) {
	rows, err := r.db.Querier(ctx).
		QueryContext(ctx, listPendingDeliveriesQuery, repositoryID, at, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending source control deliveries: %w", err)
	}

	defer func() { _ = rows.Close() }()

	deliveries := make([]entity.SCMDelivery, 0, limit)

	for rows.Next() {
		delivery, err := scanDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("scan pending source control delivery: %w", err)
		}

		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending source control deliveries: %w", err)
	}

	return deliveries, nil
}

const listDeliveriesQuery = `
SELECT` + deliveryColumns + `
FROM workspace_scm_deliveries
WHERE repository_id = $1
ORDER BY received_at DESC, id
LIMIT $2`

func (r *deliveryRepository) ListByRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
	limit int,
) ([]entity.SCMDelivery, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, listDeliveriesQuery, repositoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("list source control deliveries: %w", err)
	}

	defer func() { _ = rows.Close() }()

	deliveries := make([]entity.SCMDelivery, 0, limit)

	for rows.Next() {
		delivery, err := scanDelivery(rows)
		if err != nil {
			return nil, fmt.Errorf("scan source control delivery: %w", err)
		}

		deliveries = append(deliveries, delivery)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read source control deliveries: %w", err)
	}

	return deliveries, nil
}

const deleteSettledDeliveriesQuery = `
DELETE FROM workspace_scm_deliveries
WHERE id IN (
    SELECT id
    FROM workspace_scm_deliveries
    WHERE processed_at IS NOT NULL AND received_at < $1
    ORDER BY received_at, id
    LIMIT $2
)`

func (r *deliveryRepository) DeleteSettledBefore(
	ctx context.Context,
	before time.Time,
	limit int,
) (int, error) {
	result, err := r.db.Querier(ctx).
		ExecContext(ctx, deleteSettledDeliveriesQuery, before, limit)
	if err != nil {
		return 0, fmt.Errorf("delete settled source control deliveries: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read affected rows: %w", err)
	}

	return int(affected), nil
}
