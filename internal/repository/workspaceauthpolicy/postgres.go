package workspaceauthpolicy

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

const selectPolicyQuery = `
SELECT workspace_id, enforcement, created_at, updated_at
FROM workspace_auth_policies
WHERE workspace_id = $1`

const accountEnforcementsQuery = `
SELECT COALESCE(policy.enforcement, $2)
FROM workspace_memberships membership
LEFT JOIN workspace_auth_policies policy ON policy.workspace_id = membership.workspace_id
WHERE membership.account_id = $1
ORDER BY membership.workspace_id`

const upsertPolicyQuery = `
INSERT INTO workspace_auth_policies (workspace_id, enforcement, created_at, updated_at)
VALUES ($1, $2, $3, $3)
ON CONFLICT (workspace_id) DO UPDATE
SET enforcement = EXCLUDED.enforcement, updated_at = EXCLUDED.updated_at
RETURNING workspace_id, enforcement, created_at, updated_at`

type workspaceAuthPolicyRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.WorkspaceAuthPolicy {
	return &workspaceAuthPolicyRepository{db: db}
}

func scanPolicy(row *sql.Row) (entity.WorkspaceAuthPolicy, error) {
	var (
		workspaceID string
		enforcement string
		createdAt   time.Time
		updatedAt   time.Time
	)

	if err := row.Scan(&workspaceID, &enforcement, &createdAt, &updatedAt); err != nil {
		return entity.WorkspaceAuthPolicy{}, err
	}

	id, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.WorkspaceAuthPolicy{}, fmt.Errorf("parse workspace auth policy workspace id: %w", err)
	}

	return entity.WorkspaceAuthPolicy{
		WorkspaceID: id,
		Enforcement: entity.AuthEnforcement(enforcement),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func (r *workspaceAuthPolicyRepository) Get(ctx context.Context, workspaceID uuid.UUID) (entity.WorkspaceAuthPolicy, error) {
	policy, err := scanPolicy(r.db.Querier(ctx).QueryRowContext(ctx, selectPolicyQuery, workspaceID.String()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.DefaultWorkspaceAuthPolicy(workspaceID), nil
		}

		return entity.WorkspaceAuthPolicy{}, fmt.Errorf("find workspace auth policy: %w", err)
	}

	return policy, nil
}

func (r *workspaceAuthPolicyRepository) ListEnforcementsByAccountID(ctx context.Context, accountID uuid.UUID) ([]entity.AuthEnforcement, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, accountEnforcementsQuery,
		accountID.String(),
		string(entity.AuthEnforcementAny),
	)
	if err != nil {
		return nil, fmt.Errorf("list workspace auth enforcements: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var enforcements []entity.AuthEnforcement

	for rows.Next() {
		var enforcement string

		if err := rows.Scan(&enforcement); err != nil {
			return nil, fmt.Errorf("scan workspace auth enforcement: %w", err)
		}

		enforcements = append(enforcements, entity.AuthEnforcement(enforcement))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace auth enforcements: %w", err)
	}

	return enforcements, nil
}

func (r *workspaceAuthPolicyRepository) Upsert(ctx context.Context, policy entity.WorkspaceAuthPolicy) (entity.WorkspaceAuthPolicy, error) {
	row := r.db.Querier(ctx).QueryRowContext(ctx, upsertPolicyQuery,
		policy.WorkspaceID.String(),
		string(policy.Enforcement),
		time.Now().UTC(),
	)

	stored, err := scanPolicy(row)
	if err != nil {
		return entity.WorkspaceAuthPolicy{}, fmt.Errorf("upsert workspace auth policy: %w", err)
	}

	return stored, nil
}
