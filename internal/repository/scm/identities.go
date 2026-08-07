package scm

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type identityRepository struct {
	db *postgres.Client
}

func NewSCMIdentity(db *postgres.Client) repository.SCMIdentity {
	return &identityRepository{db: db}
}

const listIdentitiesQuery = `
SELECT id, workspace_id, account_id, provider, login, created_at
FROM workspace_scm_identities
WHERE workspace_id = $1
ORDER BY provider, lower(login)`

func (r *identityRepository) List(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.SCMIdentities, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, listIdentitiesQuery, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list platform identities: %w", err)
	}

	defer func() { _ = rows.Close() }()

	identities := make(entity.SCMIdentities, 0)

	for rows.Next() {
		var identity entity.SCMIdentity

		if err := rows.Scan(
			&identity.ID,
			&identity.WorkspaceID,
			&identity.AccountID,
			&identity.Provider,
			&identity.Login,
			&identity.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("read a platform identity: %w", err)
		}

		identities = append(identities, identity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list platform identities: %w", err)
	}

	return identities, nil
}

const insertIdentityQuery = `
INSERT INTO workspace_scm_identities (id, workspace_id, account_id, provider, login)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, workspace_id, account_id, provider, login, created_at`

func (r *identityRepository) Create(
	ctx context.Context,
	identity entity.SCMIdentity,
) (entity.SCMIdentity, error) {
	if identity.ID == uuid.Nil {
		identity.ID = uuid.New()
	}

	var created entity.SCMIdentity

	err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertIdentityQuery,
		identity.ID,
		identity.WorkspaceID,
		identity.AccountID,
		identity.Provider,
		identity.Login,
	).Scan(
		&created.ID,
		&created.WorkspaceID,
		&created.AccountID,
		&created.Provider,
		&created.Login,
		&created.CreatedAt,
	)
	if err != nil {
		if violates(err, identityLoginUniqueIndex) || violates(err, identityAccountUniqueIndex) {
			return entity.SCMIdentity{}, entity.ErrSCMIdentityExists
		}

		return entity.SCMIdentity{}, fmt.Errorf("map a platform identity: %w", err)
	}

	return created, nil
}

const deleteIdentityQuery = `
DELETE FROM workspace_scm_identities
WHERE workspace_id = $1 AND id = $2`

func (r *identityRepository) Delete(
	ctx context.Context,
	workspaceID, identityID uuid.UUID,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, deleteIdentityQuery, workspaceID, identityID,
	)
	if err != nil {
		return fmt.Errorf("unmap a platform identity: %w", err)
	}

	return expectOne(result, entity.ErrSCMIdentityNotFound)
}
