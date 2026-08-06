package webhook

import (
	"context"
	"fmt"
	"time"

	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const dropDeliveriesQuery = `
DELETE FROM webhook_deliveries
WHERE id IN (
    SELECT id FROM webhook_deliveries WHERE created_at < $1 LIMIT $2
)`

const dropOutboxQuery = `
DELETE FROM webhook_outbox
WHERE id IN (
    SELECT id FROM webhook_outbox WHERE occurred_at < $1 LIMIT $2
)`

type retentionRepository struct {
	db *postgres.Client
}

func NewRetention(db *postgres.Client) repository.WebhookRetention {
	return &retentionRepository{db: db}
}

func (r *retentionRepository) DropDeliveriesBefore(
	ctx context.Context,
	cutoff time.Time,
	batch int,
) (int, error) {
	return r.drop(ctx, dropDeliveriesQuery, "drop expired webhook deliveries", cutoff, batch)
}

func (r *retentionRepository) DropOutboxBefore(
	ctx context.Context,
	cutoff time.Time,
	batch int,
) (int, error) {
	return r.drop(ctx, dropOutboxQuery, "drop dispatched webhook events", cutoff, batch)
}

func (r *retentionRepository) drop(
	ctx context.Context,
	query, label string,
	cutoff time.Time,
	batch int,
) (int, error) {
	result, err := r.db.Querier(ctx).ExecContext(ctx, query, cutoff, batch)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}

	dropped, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", label, err)
	}

	return int(dropped), nil
}
