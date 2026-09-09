package intake

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	localPartIndex      = "workspace_team_intake_addresses_local_part_idx"
	externalIndex       = "workspace_intake_deliveries_external_idx"
)

const addressColumns = `
       team_id,
       workspace_id,
       local_part,
       domain,
       coalesce(enabled_by_account_id::text, ''),
       created_at,
       rotated_at`

const addressByTeamQuery = `
SELECT` + addressColumns + `
FROM workspace_team_intake_addresses
WHERE team_id = $1 AND workspace_id = $2`

const addressByLocalPartQuery = `
SELECT` + addressColumns + `
FROM workspace_team_intake_addresses
WHERE lower(local_part) = lower($1) AND lower(domain) = lower($2)`

const saveAddressQuery = `
INSERT INTO workspace_team_intake_addresses
    (team_id, workspace_id, local_part, domain, enabled_by_account_id, created_at, rotated_at)
VALUES ($1, $2, $3, $4, nullif($5, '')::uuid, $6, $7)
ON CONFLICT (team_id) DO UPDATE
    SET local_part            = excluded.local_part,
        domain                = excluded.domain,
        enabled_by_account_id = excluded.enabled_by_account_id,
        rotated_at            = excluded.rotated_at
RETURNING` + addressColumns

const disableAddressQuery = `
DELETE FROM workspace_team_intake_addresses
WHERE team_id = $1 AND workspace_id = $2`

const recordDeliveryQuery = `
INSERT INTO workspace_intake_deliveries
    (id, workspace_id, team_id, external_id, recipient, sender, subject, received_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id`

const deliveryByIDQuery = `
SELECT id,
       workspace_id,
       team_id,
       external_id,
       recipient,
       sender,
       subject,
       received_at,
       processed_at,
       outcome,
       coalesce(issue_id::text, ''),
       failure
FROM workspace_intake_deliveries
WHERE id = $1`

const settleDeliveryQuery = `
UPDATE workspace_intake_deliveries
SET outcome      = $2,
    issue_id     = nullif($3, '')::uuid,
    failure      = $4,
    processed_at = $5
WHERE id = $1`

type intakeRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Intake {
	return &intakeRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAddress(row scanner) (entity.IntakeAddress, error) {
	var (
		address                  entity.IntakeAddress
		team, workspace, enabler string
		rotated                  sql.NullTime
	)

	if err := row.Scan(
		&team, &workspace, &address.LocalPart, &address.Domain, &enabler,
		&address.CreatedAt, &rotated,
	); err != nil {
		return entity.IntakeAddress{}, err
	}

	parsed, err := uuid.Parse(team)
	if err != nil {
		return entity.IntakeAddress{}, fmt.Errorf("parse intake team id: %w", err)
	}

	address.TeamID = parsed

	if address.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.IntakeAddress{}, fmt.Errorf("parse intake workspace id: %w", err)
	}

	if enabler != "" {
		if address.EnabledBy, err = uuid.Parse(enabler); err != nil {
			return entity.IntakeAddress{}, fmt.Errorf("parse intake enabler id: %w", err)
		}
	}

	if rotated.Valid {
		at := rotated.Time.UTC()
		address.RotatedAt = &at
	}

	return address, nil
}

func (r *intakeRepository) Address(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (entity.IntakeAddress, error) {
	address, err := scanAddress(r.db.Querier(ctx).QueryRowContext(
		ctx, addressByTeamQuery, teamID.String(), workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IntakeAddress{}, entity.ErrIntakeDisabled
		}

		return entity.IntakeAddress{}, fmt.Errorf("find intake address: %w", err)
	}

	return address, nil
}

func (r *intakeRepository) AddressOf(
	ctx context.Context,
	localPart, domain string,
) (entity.IntakeAddress, error) {
	address, err := scanAddress(r.db.Querier(ctx).QueryRowContext(
		ctx, addressByLocalPartQuery, localPart, domain,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IntakeAddress{}, entity.ErrIntakeAddressUnknown
		}

		return entity.IntakeAddress{}, fmt.Errorf("find intake address by local part: %w", err)
	}

	return address, nil
}

func (r *intakeRepository) Save(
	ctx context.Context,
	address entity.IntakeAddress,
) (entity.IntakeAddress, error) {
	rotated := sql.NullTime{}
	if address.RotatedAt != nil {
		rotated = sql.NullTime{Time: *address.RotatedAt, Valid: true}
	}

	created := address.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}

	saved, err := scanAddress(r.db.Querier(ctx).QueryRowContext(
		ctx, saveAddressQuery,
		address.TeamID.String(), address.WorkspaceID.String(),
		address.LocalPart, address.Domain, enabler(address.EnabledBy), created, rotated,
	))
	if err != nil {
		if violates(err, localPartIndex) {
			return entity.IntakeAddress{}, entity.ErrIntakeAddressTaken
		}

		return entity.IntakeAddress{}, fmt.Errorf("save intake address: %w", err)
	}

	return saved, nil
}

func (r *intakeRepository) Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, disableAddressQuery, teamID.String(), workspaceID.String(),
	); err != nil {
		return fmt.Errorf("disable intake address: %w", err)
	}

	return nil
}

func (r *intakeRepository) Record(
	ctx context.Context,
	delivery entity.IntakeDelivery,
) (uuid.UUID, error) {
	id := delivery.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	var stored string

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, recordDeliveryQuery,
		id.String(), delivery.WorkspaceID.String(), delivery.TeamID.String(),
		delivery.ExternalID, delivery.Recipient, delivery.Sender, delivery.Subject,
		delivery.ReceivedAt,
	).Scan(&stored); err != nil {
		if violates(err, externalIndex) {
			return uuid.Nil, entity.ErrIntakeDeliveryDuplicate
		}

		return uuid.Nil, fmt.Errorf("record intake delivery: %w", err)
	}

	parsed, err := uuid.Parse(stored)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse intake delivery id: %w", err)
	}

	return parsed, nil
}

func (r *intakeRepository) Delivery(
	ctx context.Context,
	deliveryID uuid.UUID,
) (entity.IntakeDelivery, error) {
	var (
		delivery                     entity.IntakeDelivery
		id, workspace, team, issueID string
		processed                    sql.NullTime
		outcome                      string
	)

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, deliveryByIDQuery, deliveryID.String(),
	).Scan(
		&id, &workspace, &team, &delivery.ExternalID, &delivery.Recipient, &delivery.Sender,
		&delivery.Subject, &delivery.ReceivedAt, &processed, &outcome, &issueID, &delivery.Failure,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IntakeDelivery{}, entity.ErrIntakeDeliveryNotFound
		}

		return entity.IntakeDelivery{}, fmt.Errorf("find intake delivery: %w", err)
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.IntakeDelivery{}, fmt.Errorf("parse intake delivery id: %w", err)
	}

	delivery.ID = parsed

	if delivery.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.IntakeDelivery{}, fmt.Errorf("parse intake delivery workspace id: %w", err)
	}

	if delivery.TeamID, err = uuid.Parse(team); err != nil {
		return entity.IntakeDelivery{}, fmt.Errorf("parse intake delivery team id: %w", err)
	}

	if issueID != "" {
		if delivery.IssueID, err = uuid.Parse(issueID); err != nil {
			return entity.IntakeDelivery{}, fmt.Errorf("parse intake delivery issue id: %w", err)
		}
	}

	if processed.Valid {
		at := processed.Time.UTC()
		delivery.ProcessedAt = &at
	}

	delivery.Outcome = entity.IntakeDeliveryOutcome(outcome)

	return delivery, nil
}

func (r *intakeRepository) Settle(
	ctx context.Context,
	deliveryID uuid.UUID,
	outcome entity.IntakeDeliveryOutcome,
	issueID uuid.UUID,
	failure string,
	at time.Time,
) error {
	issue := ""
	if issueID != uuid.Nil {
		issue = issueID.String()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, settleDeliveryQuery, deliveryID.String(), string(outcome), issue, failure, at,
	); err != nil {
		return fmt.Errorf("settle intake delivery: %w", err)
	}

	return nil
}

func enabler(accountID uuid.UUID) string {
	if accountID == uuid.Nil {
		return ""
	}

	return accountID.String()
}

func violates(err error, index string) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolationCode &&
		pgErr.ConstraintName == index
}
