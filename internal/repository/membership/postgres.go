package membership

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
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

const membershipPageQuery = `
SELECT m.id,
       m.workspace_id,
       m.account_id,
       m.role,
       m.source,
       m.last_active_at,
       m.last_auth_method,
       m.created_at,
       m.updated_at,
       a.kind AS account_kind,
       coalesce(a.display_name, '') AS display_name,
       coalesce(a.email, '') AS email,
       lower(coalesce(a.display_name, '')) AS sort_name
FROM workspace_memberships m
JOIN accounts a ON a.id = m.account_id
WHERE m.workspace_id = $1
  AND ($2 = ''
       OR position(lower($2) IN lower(coalesce(a.display_name, ''))) > 0
       OR position(lower($2) IN lower(coalesce(a.email, ''))) > 0)
  AND ($3::boolean IS NOT TRUE
       OR (lower(coalesce(a.display_name, '')), m.account_id) > ($4, $5::uuid))
ORDER BY lower(coalesce(a.display_name, '')), m.account_id
LIMIT $6`

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

	membership := entity.Membership{
		ID:          id,
		WorkspaceID: workspaceID,
		AccountID:   accountID,
		Role:        entity.MembershipRole(model.Role),
		Source:      entity.MembershipSource(model.Source),
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}

	if model.LastActiveAt.Valid {
		lastActiveAt := model.LastActiveAt.Time
		membership.LastActiveAt = &lastActiveAt
	}

	if model.LastAuthMethod.Valid {
		membership.LastAuthMethod = entity.SessionAuthMethod(model.LastAuthMethod.String)
	}

	return membership, nil
}

func toModel(membership entity.Membership) *dbpostgres.WorkspaceMembership {
	model := &dbpostgres.WorkspaceMembership{
		ID:          membership.ID.String(),
		WorkspaceID: membership.WorkspaceID.String(),
		AccountID:   membership.AccountID.String(),
		Role:        string(membership.Role),
		Source:      string(membership.Source),
		CreatedAt:   membership.CreatedAt,
		UpdatedAt:   membership.UpdatedAt,
	}

	if membership.LastActiveAt != nil {
		model.LastActiveAt = null.TimeFrom(*membership.LastActiveAt)
	}

	if membership.LastAuthMethod != "" {
		model.LastAuthMethod = null.StringFrom(string(membership.LastAuthMethod))
	}

	return model
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

func (r *membershipRepository) ListPageByWorkspaceID(
	ctx context.Context,
	workspaceID uuid.UUID,
	page entity.MembershipPage,
) ([]entity.WorkspaceMember, error) {
	cursorName := ""
	cursorAccountID := uuid.Nil.String()

	if page.Cursor != nil {
		cursorName = page.Cursor.Name
		cursorAccountID = page.Cursor.AccountID.String()
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		membershipPageQuery,
		workspaceID.String(),
		page.Query,
		page.Cursor != nil,
		cursorName,
		cursorAccountID,
		page.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}
	defer func() { _ = rows.Close() }()

	members := make([]entity.WorkspaceMember, 0, page.Limit)

	for rows.Next() {
		member, err := scanWorkspaceMember(rows)
		if err != nil {
			return nil, err
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan workspace members: %w", err)
	}

	return members, nil
}

func scanWorkspaceMember(rows *sql.Rows) (entity.WorkspaceMember, error) {
	var (
		rawID          string
		rawWorkspaceID string
		rawAccountID   string
		role           string
		source         string
		lastActiveAt   sql.NullTime
		lastAuthMethod sql.NullString
		createdAt      time.Time
		updatedAt      time.Time
		accountKind    string
		displayName    string
		email          string
		sortName       string
	)

	if err := rows.Scan(
		&rawID,
		&rawWorkspaceID,
		&rawAccountID,
		&role,
		&source,
		&lastActiveAt,
		&lastAuthMethod,
		&createdAt,
		&updatedAt,
		&accountKind,
		&displayName,
		&email,
		&sortName,
	); err != nil {
		return entity.WorkspaceMember{}, fmt.Errorf("scan workspace member: %w", err)
	}

	id, err := uuid.Parse(rawID)
	if err != nil {
		return entity.WorkspaceMember{}, fmt.Errorf("parse membership id: %w", err)
	}

	workspaceID, err := uuid.Parse(rawWorkspaceID)
	if err != nil {
		return entity.WorkspaceMember{}, fmt.Errorf("parse membership workspace id: %w", err)
	}

	accountID, err := uuid.Parse(rawAccountID)
	if err != nil {
		return entity.WorkspaceMember{}, fmt.Errorf("parse membership account id: %w", err)
	}

	membership := entity.Membership{
		ID:          id,
		WorkspaceID: workspaceID,
		AccountID:   accountID,
		Role:        entity.MembershipRole(role),
		Source:      entity.MembershipSource(source),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	if lastActiveAt.Valid {
		activeAt := lastActiveAt.Time
		membership.LastActiveAt = &activeAt
	}

	if lastAuthMethod.Valid {
		membership.LastAuthMethod = entity.SessionAuthMethod(lastAuthMethod.String)
	}

	return entity.WorkspaceMember{
		AccountKind: entity.AccountKind(accountKind),
		Membership:  membership,
		DisplayName: displayName,
		Email:       email,
		SortName:    sortName,
	}, nil
}

func (r *membershipRepository) RecordActivity(
	ctx context.Context,
	accountID uuid.UUID,
	activeAt time.Time,
	method entity.SessionAuthMethod,
) error {
	if _, err := dbpostgres.WorkspaceMemberships(
		dbpostgres.WorkspaceMembershipWhere.AccountID.EQ(accountID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceMembershipColumns.LastActiveAt:   null.TimeFrom(activeAt),
		dbpostgres.WorkspaceMembershipColumns.LastAuthMethod: null.StringFrom(string(method)),
	}); err != nil {
		return fmt.Errorf("record membership activity: %w", err)
	}

	return nil
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
