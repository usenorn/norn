package membership

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	memberUniqueIndex   = "workspace_memberships_workspace_account_key"
)

const adminWorkspaceIDsQuery = `
SELECT workspace_id
FROM workspace_memberships
WHERE account_id = $1 AND role = $2
ORDER BY workspace_id`

const soleAdminWorkspaceIDsQuery = `
SELECT held.workspace_id
FROM workspace_memberships held
WHERE held.account_id = $1
  AND held.role = $2
  AND NOT EXISTS (
      SELECT 1
      FROM workspace_memberships peer
      JOIN accounts peer_account ON peer_account.id = peer.account_id
      WHERE peer.workspace_id = held.workspace_id
        AND peer.account_id <> held.account_id
        AND peer.role = $2
        AND peer_account.status = $3
  )
ORDER BY held.workspace_id`

func toEntity(model *dbpostgres.WorkspaceMembership) (entity.Membership, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.Membership{}, fmt.Errorf("parse membership id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.Membership{}, fmt.Errorf("parse membership workspace id: %w", err)
	}

	accountID, err := uuid.Parse(model.AccountID)
	if err != nil {
		return entity.Membership{}, fmt.Errorf("parse membership account id: %w", err)
	}

	return entity.Membership{
		ID:          id,
		WorkspaceID: workspaceID,
		AccountID:   accountID,
		Role:        entity.MembershipRole(model.Role),
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func toModel(membership entity.Membership) *dbpostgres.WorkspaceMembership {
	return &dbpostgres.WorkspaceMembership{
		ID:          membership.ID.String(),
		WorkspaceID: membership.WorkspaceID.String(),
		AccountID:   membership.AccountID.String(),
		Role:        string(membership.Role),
		CreatedAt:   membership.CreatedAt,
		UpdatedAt:   membership.UpdatedAt,
	}
}

type membershipRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Membership {
	return &membershipRepository{db: db}
}

func (r *membershipRepository) Create(ctx context.Context, membership entity.Membership) (entity.Membership, error) {
	if membership.ID == uuid.Nil {
		membership.ID = uuid.New()
	}

	now := time.Now().UTC()
	membership.CreatedAt = now
	membership.UpdatedAt = now

	model := toModel(membership)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == memberUniqueIndex {
			return entity.Membership{}, entity.ErrMembershipExists
		}

		return entity.Membership{}, fmt.Errorf("insert membership: %w", err)
	}

	return toEntity(model)
}

func (r *membershipRepository) Get(ctx context.Context, workspaceID, accountID uuid.UUID) (entity.Membership, error) {
	model, err := dbpostgres.WorkspaceMemberships(
		dbpostgres.WorkspaceMembershipWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceMembershipWhere.AccountID.EQ(accountID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Membership{}, entity.ErrMembershipNotFound
		}

		return entity.Membership{}, fmt.Errorf("find membership: %w", err)
	}

	return toEntity(model)
}

func (r *membershipRepository) ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]entity.Membership, error) {
	models, err := dbpostgres.WorkspaceMemberships(
		dbpostgres.WorkspaceMembershipWhere.WorkspaceID.EQ(workspaceID.String()),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list memberships by workspace: %w", err)
	}

	memberships := make([]entity.Membership, 0, len(models))

	for _, model := range models {
		membership, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		memberships = append(memberships, membership)
	}

	return memberships, nil
}

func (r *membershipRepository) UpdateRole(ctx context.Context, workspaceID, accountID uuid.UUID, role entity.MembershipRole) (entity.Membership, error) {
	updated, err := dbpostgres.WorkspaceMemberships(
		dbpostgres.WorkspaceMembershipWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceMembershipWhere.AccountID.EQ(accountID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceMembershipColumns.Role:      string(role),
		dbpostgres.WorkspaceMembershipColumns.UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return entity.Membership{}, fmt.Errorf("update membership role: %w", err)
	}

	if updated == 0 {
		return entity.Membership{}, entity.ErrMembershipNotFound
	}

	return r.Get(ctx, workspaceID, accountID)
}

func (r *membershipRepository) Delete(ctx context.Context, workspaceID, accountID uuid.UUID) error {
	deleted, err := dbpostgres.WorkspaceMemberships(
		dbpostgres.WorkspaceMembershipWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceMembershipWhere.AccountID.EQ(accountID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete membership: %w", err)
	}

	if deleted == 0 {
		return entity.ErrMembershipNotFound
	}

	return nil
}

func (r *membershipRepository) DeleteByAccountID(ctx context.Context, accountID uuid.UUID) error {
	if _, err := dbpostgres.WorkspaceMemberships(
		dbpostgres.WorkspaceMembershipWhere.AccountID.EQ(accountID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx)); err != nil {
		return fmt.Errorf("delete memberships by account: %w", err)
	}

	return nil
}

func (r *membershipRepository) ListAdminWorkspaceIDs(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	return r.queryWorkspaceIDs(ctx, adminWorkspaceIDsQuery, accountID.String(), string(entity.MembershipRoleAdmin))
}

func (r *membershipRepository) ListWorkspaceIDsWithoutOtherActiveAdmin(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	return r.queryWorkspaceIDs(ctx, soleAdminWorkspaceIDsQuery,
		accountID.String(),
		string(entity.MembershipRoleAdmin),
		string(entity.AccountStatusActive),
	)
}

func (r *membershipRepository) queryWorkspaceIDs(ctx context.Context, query string, arguments ...any) ([]uuid.UUID, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query workspace ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []uuid.UUID

	for rows.Next() {
		var raw string

		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan workspace id: %w", err)
		}

		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("parse workspace id: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query workspace ids: %w", err)
	}

	return ids, nil
}
