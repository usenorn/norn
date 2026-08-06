package webhook

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/crypter"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const webhookColumns = `
	w.id, w.workspace_id, w.owner_account_id, w.name, w.url, w.events,
	w.secret_hint, w.enabled, w.disabled_at, w.disabled_reason,
	w.failure_streak, w.last_delivered_at, w.created_at, w.updated_at`

const secretColumns = `, w.secret_sealed, w.previous_secret_sealed, w.previous_secret_expires_at`

const insertWebhookQuery = `
INSERT INTO workspace_webhooks (
    id, workspace_id, owner_account_id, name, url, events, secret_sealed, secret_hint
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

const updateWebhookQuery = `
UPDATE workspace_webhooks
SET name = $2, url = $3, events = $4, updated_at = now()
WHERE id = $1`

const rotateSecretQuery = `
UPDATE workspace_webhooks
SET previous_secret_sealed = secret_sealed,
    previous_secret_expires_at = $3,
    secret_sealed = $2,
    secret_hint = $4,
    updated_at = now()
WHERE id = $1`

type webhookRepository struct {
	db      *postgres.Client
	crypter *crypter.Crypter
}

func New(db *postgres.Client, sealer *crypter.Crypter) repository.Webhook {
	return &webhookRepository{db: db, crypter: sealer}
}

func (r *webhookRepository) seal(secret string) ([]byte, error) {
	sealed, err := r.crypter.Seal([]byte(secret))
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return nil, entity.ErrWebhookEncryptionKeyMissing
		}

		return nil, fmt.Errorf("seal webhook secret: %w", err)
	}

	return sealed, nil
}

func (r *webhookRepository) open(sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}

	secret, err := r.crypter.Open(sealed)
	if err != nil {
		if errors.Is(err, crypter.ErrKeyMissing) {
			return "", entity.ErrWebhookEncryptionKeyMissing
		}

		return "", fmt.Errorf("open stored webhook secret: %w", err)
	}

	return string(secret), nil
}

func (r *webhookRepository) Create(ctx context.Context, hook entity.Webhook) (entity.Webhook, error) {
	if hook.ID == uuid.Nil {
		hook.ID = uuid.New()
	}

	sealed, err := r.seal(hook.Secret)
	if err != nil {
		return entity.Webhook{}, err
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		insertWebhookQuery,
		hook.ID.String(),
		hook.WorkspaceID.String(),
		hook.OwnerAccountID.String(),
		hook.Name,
		hook.URL,
		types.StringArray(entity.WebhookEventStrings(hook.Events)),
		sealed,
		entity.WebhookSecretHint(hook.Secret),
	); err != nil {
		return entity.Webhook{}, fmt.Errorf("insert webhook: %w", err)
	}

	return r.Get(ctx, hook.WorkspaceID, hook.ID)
}

func (r *webhookRepository) Get(ctx context.Context, workspaceID, webhookID uuid.UUID) (entity.Webhook, error) {
	return r.one(
		ctx,
		`SELECT`+webhookColumns+` FROM workspace_webhooks w WHERE w.workspace_id = $1 AND w.id = $2`,
		workspaceID.String(), webhookID.String(),
	)
}

func (r *webhookRepository) GetForDelivery(ctx context.Context, webhookID uuid.UUID) (entity.Webhook, error) {
	var (
		hook                     entity.Webhook
		sealed, previouslySealed []byte
		expires                  sql.NullTime
	)

	row := r.db.Querier(ctx).QueryRowContext(
		ctx,
		`SELECT`+webhookColumns+secretColumns+` FROM workspace_webhooks w WHERE w.id = $1`,
		webhookID.String(),
	)

	scanned, err := scanWebhookRow(row, &hook, &sealed, &previouslySealed, &expires)
	if err != nil {
		return entity.Webhook{}, err
	}

	if scanned.Secret, err = r.open(sealed); err != nil {
		return entity.Webhook{}, err
	}

	if scanned.PreviousSecret, err = r.open(previouslySealed); err != nil {
		return entity.Webhook{}, err
	}

	if expires.Valid {
		scanned.SecretExpiresAt = &expires.Time
	}

	return scanned, nil
}

func (r *webhookRepository) ListByWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.Webhook, error) {
	return r.many(
		ctx,
		`SELECT`+webhookColumns+`
		FROM workspace_webhooks w
		WHERE w.workspace_id = $1
		ORDER BY w.created_at DESC, w.id`,
		workspaceID.String(),
	)
}

func (r *webhookRepository) ListSubscribed(
	ctx context.Context,
	workspaceID uuid.UUID,
	event entity.WebhookEvent,
) ([]entity.Webhook, error) {
	return r.many(
		ctx,
		`SELECT`+webhookColumns+`
		FROM workspace_webhooks w
		WHERE w.workspace_id = $1 AND w.enabled AND w.events @> ARRAY[$2]::text[]
		ORDER BY w.created_at, w.id`,
		workspaceID.String(), string(event),
	)
}

func (r *webhookRepository) Update(ctx context.Context, hook entity.Webhook) (entity.Webhook, error) {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		updateWebhookQuery,
		hook.ID.String(),
		hook.Name,
		hook.URL,
		types.StringArray(entity.WebhookEventStrings(hook.Events)),
	)
	if err != nil {
		return entity.Webhook{}, fmt.Errorf("update webhook: %w", err)
	}

	if err := expectOne(result, entity.ErrWebhookNotFound); err != nil {
		return entity.Webhook{}, err
	}

	return r.Get(ctx, hook.WorkspaceID, hook.ID)
}

func (r *webhookRepository) RotateSecret(
	ctx context.Context,
	webhookID uuid.UUID,
	secret string,
	graceUntil time.Time,
) error {
	sealed, err := r.seal(secret)
	if err != nil {
		return err
	}

	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		rotateSecretQuery,
		webhookID.String(),
		sealed,
		graceUntil,
		entity.WebhookSecretHint(secret),
	)
	if err != nil {
		return fmt.Errorf("rotate webhook secret: %w", err)
	}

	return expectOne(result, entity.ErrWebhookNotFound)
}

func (r *webhookRepository) RecordSuccess(ctx context.Context, webhookID uuid.UUID, at time.Time) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE workspace_webhooks
		 SET failure_streak = 0, last_delivered_at = $2, updated_at = now()
		 WHERE id = $1`,
		webhookID.String(), at,
	); err != nil {
		return fmt.Errorf("record webhook delivery: %w", err)
	}

	return nil
}

func (r *webhookRepository) RecordFailure(ctx context.Context, webhookID uuid.UUID) (int, error) {
	var streak int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		`UPDATE workspace_webhooks
		 SET failure_streak = failure_streak + 1, updated_at = now()
		 WHERE id = $1
		 RETURNING failure_streak`,
		webhookID.String(),
	).Scan(&streak); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, entity.ErrWebhookNotFound
		}

		return 0, fmt.Errorf("record webhook failure: %w", err)
	}

	return streak, nil
}

func (r *webhookRepository) Disable(
	ctx context.Context,
	webhookID uuid.UUID,
	reason entity.WebhookDisableReason,
	at time.Time,
) (bool, error) {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE workspace_webhooks
		 SET enabled = false, disabled_at = $3, disabled_reason = $2, updated_at = now()
		 WHERE id = $1 AND enabled`,
		webhookID.String(), string(reason), at,
	)
	if err != nil {
		return false, fmt.Errorf("disable webhook: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("disable webhook: %w", err)
	}

	return affected > 0, nil
}

func (r *webhookRepository) Enable(ctx context.Context, webhookID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`UPDATE workspace_webhooks
		 SET enabled = true, disabled_at = NULL, disabled_reason = '', failure_streak = 0, updated_at = now()
		 WHERE id = $1`,
		webhookID.String(),
	)
	if err != nil {
		return fmt.Errorf("enable webhook: %w", err)
	}

	return expectOne(result, entity.ErrWebhookNotFound)
}

func (r *webhookRepository) Delete(ctx context.Context, workspaceID, webhookID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		`DELETE FROM workspace_webhooks WHERE workspace_id = $1 AND id = $2`,
		workspaceID.String(), webhookID.String(),
	)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}

	return expectOne(result, entity.ErrWebhookNotFound)
}

func (r *webhookRepository) one(ctx context.Context, query string, args ...any) (entity.Webhook, error) {
	hooks, err := r.many(ctx, query, args...)
	if err != nil {
		return entity.Webhook{}, err
	}

	if len(hooks) == 0 {
		return entity.Webhook{}, entity.ErrWebhookNotFound
	}

	return hooks[0], nil
}

func (r *webhookRepository) many(ctx context.Context, query string, args ...any) ([]entity.Webhook, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query webhooks: %w", err)
	}

	defer func() { _ = rows.Close() }()

	hooks := make([]entity.Webhook, 0)

	for rows.Next() {
		hook, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}

		hooks = append(hooks, hook)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read webhooks: %w", err)
	}

	return hooks, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanWebhook(row scanner) (entity.Webhook, error) {
	var hook entity.Webhook

	scanned, err := scanWebhookRow(row, &hook, nil, nil, nil)

	return scanned, err
}

func scanWebhookRow(
	row scanner,
	hook *entity.Webhook,
	sealed, previouslySealed *[]byte,
	expires *sql.NullTime,
) (entity.Webhook, error) {
	var (
		rawID, rawWorkspace, rawOwner string
		events                        types.StringArray
		disabledReason                string
		disabledAt, lastDeliveredAt   sql.NullTime
	)

	targets := []any{
		&rawID, &rawWorkspace, &rawOwner, &hook.Name, &hook.URL, &events,
		&hook.SecretHint, &hook.Enabled, &disabledAt, &disabledReason,
		&hook.FailureStreak, &lastDeliveredAt, &hook.CreatedAt, &hook.UpdatedAt,
	}

	if sealed != nil {
		targets = append(targets, sealed, previouslySealed, expires)
	}

	if err := row.Scan(targets...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Webhook{}, entity.ErrWebhookNotFound
		}

		return entity.Webhook{}, fmt.Errorf("scan webhook: %w", err)
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return entity.Webhook{}, fmt.Errorf("parse webhook id: %w", err)
	}

	workspaceID, err := uuid.Parse(rawWorkspace)
	if err != nil {
		return entity.Webhook{}, fmt.Errorf("parse webhook workspace id: %w", err)
	}

	ownerID, err := uuid.Parse(rawOwner)
	if err != nil {
		return entity.Webhook{}, fmt.Errorf("parse webhook owner id: %w", err)
	}

	hook.ID = id
	hook.WorkspaceID = workspaceID
	hook.OwnerAccountID = ownerID
	hook.Events = entity.NewWebhookEvents(events)
	hook.DisabledReason = entity.WebhookDisableReason(disabledReason)

	if disabledAt.Valid {
		hook.DisabledAt = &disabledAt.Time
	}

	if lastDeliveredAt.Valid {
		hook.LastDeliveredAt = &lastDeliveredAt.Time
	}

	return *hook, nil
}

func expectOne(result sql.Result, absent error) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read affected rows: %w", err)
	}

	if affected == 0 {
		return absent
	}

	return nil
}

func optionalID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}

	return id.String()
}

func parseID(raw string) uuid.UUID {
	if raw == "" {
		return uuid.Nil
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil
	}

	return parsed
}
