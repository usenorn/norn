package ssoidentity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const linkQuery = `
INSERT INTO workspace_sso_identities (workspace_id, account_id, subject)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, account_id) DO UPDATE SET subject = excluded.subject`

const identityQuery = `
SELECT workspace_id, account_id, subject, linked_at
FROM workspace_sso_identities
WHERE workspace_id = $1 AND account_id = $2`

const identitiesQuery = `
SELECT workspace_id, account_id, subject, linked_at
FROM workspace_sso_identities
WHERE workspace_id = $1
ORDER BY linked_at`

const unlinkQuery = `DELETE FROM workspace_sso_identities WHERE workspace_id = $1 AND account_id = $2`

const linkedAdminQuery = `
SELECT EXISTS (
    SELECT 1
    FROM workspace_sso_identities i
    JOIN workspace_memberships m
      ON m.workspace_id = i.workspace_id AND m.account_id = i.account_id
    JOIN accounts a ON a.id = i.account_id
    WHERE i.workspace_id = $1 AND m.role = $2 AND a.status = $3
)`

type identityRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.SSOIdentity {
	return &identityRepository{db: db}
}

func (r *identityRepository) Link(ctx context.Context, identity entity.SSOIdentity) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		linkQuery,
		identity.WorkspaceID.String(),
		identity.AccountID.String(),
		identity.Subject,
	); err != nil {
		return fmt.Errorf("link sso identity: %w", err)
	}

	return nil
}

func (r *identityRepository) Get(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
) (entity.SSOIdentity, error) {
	identity, err := scan(r.db.Querier(ctx).QueryRowContext(
		ctx,
		identityQuery,
		workspaceID.String(),
		accountID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.SSOIdentity{}, entity.ErrSSOIdentityNotFound
		}

		return entity.SSOIdentity{}, fmt.Errorf("find sso identity: %w", err)
	}

	return identity, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scan(row scanner) (entity.SSOIdentity, error) {
	var (
		identity  entity.SSOIdentity
		workspace string
		account   string
	)

	if err := row.Scan(&workspace, &account, &identity.Subject, &identity.LinkedAt); err != nil {
		return entity.SSOIdentity{}, err
	}

	parsed, err := uuid.Parse(workspace)
	if err != nil {
		return entity.SSOIdentity{}, fmt.Errorf("parse sso identity workspace id: %w", err)
	}

	identity.WorkspaceID = parsed

	if identity.AccountID, err = uuid.Parse(account); err != nil {
		return entity.SSOIdentity{}, fmt.Errorf("parse sso identity account id: %w", err)
	}

	return identity, nil
}

func (r *identityRepository) ListByWorkspaceID(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.SSOIdentity, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, identitiesQuery, workspaceID.String())
	if err != nil {
		return nil, fmt.Errorf("list sso identities: %w", err)
	}

	defer func() { _ = rows.Close() }()

	identities := make([]entity.SSOIdentity, 0)

	for rows.Next() {
		identity, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sso identity: %w", err)
		}

		identities = append(identities, identity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sso identities: %w", err)
	}

	return identities, nil
}

func (r *identityRepository) Unlink(ctx context.Context, workspaceID, accountID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx,
		unlinkQuery,
		workspaceID.String(),
		accountID.String(),
	)
	if err != nil {
		return fmt.Errorf("unlink sso identity: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read unlinked sso identity rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrSSOIdentityNotFound
	}

	return nil
}

func (r *identityRepository) AnyLinkedAdmin(
	ctx context.Context,
	workspaceID uuid.UUID,
) (bool, error) {
	var linked bool

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		linkedAdminQuery,
		workspaceID.String(),
		string(entity.MembershipRoleAdmin),
		string(entity.AccountStatusActive),
	).Scan(&linked); err != nil {
		return false, fmt.Errorf("check for a linked administrator: %w", err)
	}

	return linked, nil
}
