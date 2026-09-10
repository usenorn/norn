package requestkey

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const uniqueViolationCode = "23505"

const claimKeyQuery = `
INSERT INTO workspace_request_keys (id, workspace_id, account_id, scope, request_key, created_at)
VALUES ($1, $2, nullif($3, '')::uuid, $4, $5, $6)`

const settleKeyQuery = `
UPDATE workspace_request_keys
SET issue_id = $3
WHERE id = $1 AND workspace_id = $2`

const findKeyQuery = `
SELECT id, coalesce(issue_id::text, ''), created_at
FROM workspace_request_keys
WHERE workspace_id = $1
  AND scope = $2
  AND coalesce(account_id, '00000000-0000-0000-0000-000000000000'::uuid) = $3::uuid
  AND request_key = $4`

type requestKeyRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.RequestKey {
	return &requestKeyRepository{db: db}
}

func (r *requestKeyRepository) Claim(ctx context.Context, key entity.RequestKey) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, claimKeyQuery,
		key.ID.String(), key.WorkspaceID.String(), account(key.AccountID),
		string(key.Scope), key.Key, key.CreatedAt,
	); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return entity.ErrRequestKeyTaken
		}

		return fmt.Errorf("claim request key: %w", err)
	}

	return nil
}

func (r *requestKeyRepository) Settle(
	ctx context.Context,
	workspaceID, keyID, issueID uuid.UUID,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, settleKeyQuery, keyID.String(), workspaceID.String(), issueID.String(),
	); err != nil {
		return fmt.Errorf("settle request key: %w", err)
	}

	return nil
}

func (r *requestKeyRepository) Find(
	ctx context.Context,
	key entity.RequestKey,
) (entity.RequestKey, error) {
	found := key

	var id, issue string

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, findKeyQuery,
		key.WorkspaceID.String(), string(key.Scope), holder(key.AccountID), key.Key,
	).Scan(&id, &issue, &found.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.RequestKey{}, entity.ErrRequestKeyNotFound
		}

		return entity.RequestKey{}, fmt.Errorf("find request key: %w", err)
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.RequestKey{}, fmt.Errorf("parse request key id: %w", err)
	}

	found.ID = parsed

	if issue != "" {
		if found.IssueID, err = uuid.Parse(issue); err != nil {
			return entity.RequestKey{}, fmt.Errorf("parse request key issue id: %w", err)
		}
	}

	return found, nil
}

func account(accountID uuid.UUID) string {
	if accountID == uuid.Nil {
		return ""
	}

	return accountID.String()
}

func holder(accountID uuid.UUID) string {
	return accountID.String()
}
