package teammember

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode     = "23505"
	foreignKeyViolationCode = "23503"
	memberUniqueIndex       = "workspace_team_members_team_account_key"
	membershipForeignKey    = "workspace_team_members_membership_fkey"
)

func toEntity(model *dbpostgres.WorkspaceTeamMember) (entity.TeamMembership, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.TeamMembership{}, fmt.Errorf("parse team membership id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.TeamMembership{}, fmt.Errorf("parse team membership workspace id: %w", err)
	}

	teamID, err := uuid.Parse(model.TeamID)
	if err != nil {
		return entity.TeamMembership{}, fmt.Errorf("parse team membership team id: %w", err)
	}

	accountID, err := uuid.Parse(model.AccountID)
	if err != nil {
		return entity.TeamMembership{}, fmt.Errorf("parse team membership account id: %w", err)
	}

	return entity.TeamMembership{
		ID:          id,
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		AccountID:   accountID,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func toModel(membership entity.TeamMembership) *dbpostgres.WorkspaceTeamMember {
	return &dbpostgres.WorkspaceTeamMember{
		ID:          membership.ID.String(),
		WorkspaceID: membership.WorkspaceID.String(),
		TeamID:      membership.TeamID.String(),
		AccountID:   membership.AccountID.String(),
		CreatedAt:   membership.CreatedAt,
		UpdatedAt:   membership.UpdatedAt,
	}
}

type teamMemberRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.TeamMember {
	return &teamMemberRepository{db: db}
}

func (r *teamMemberRepository) Create(ctx context.Context, membership entity.TeamMembership) (entity.TeamMembership, error) {
	if membership.ID == uuid.Nil {
		membership.ID = uuid.New()
	}

	now := time.Now().UTC()
	membership.CreatedAt = now
	membership.UpdatedAt = now

	model := toModel(membership)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == memberUniqueIndex:
				return entity.TeamMembership{}, entity.ErrTeamMembershipExists
			case pgErr.Code == foreignKeyViolationCode && pgErr.ConstraintName == membershipForeignKey:
				return entity.TeamMembership{}, entity.ErrMembershipNotFound
			}
		}

		return entity.TeamMembership{}, fmt.Errorf("insert team membership: %w", err)
	}

	return toEntity(model)
}

func (r *teamMemberRepository) Get(ctx context.Context, teamID, accountID uuid.UUID) (entity.TeamMembership, error) {
	model, err := dbpostgres.WorkspaceTeamMembers(
		dbpostgres.WorkspaceTeamMemberWhere.TeamID.EQ(teamID.String()),
		dbpostgres.WorkspaceTeamMemberWhere.AccountID.EQ(accountID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.TeamMembership{}, entity.ErrTeamMembershipNotFound
		}

		return entity.TeamMembership{}, fmt.Errorf("find team membership: %w", err)
	}

	return toEntity(model)
}

func (r *teamMemberRepository) ListByTeamID(ctx context.Context, teamID uuid.UUID) ([]entity.TeamMembership, error) {
	models, err := dbpostgres.WorkspaceTeamMembers(
		dbpostgres.WorkspaceTeamMemberWhere.TeamID.EQ(teamID.String()),
		qm.OrderBy(dbpostgres.WorkspaceTeamMemberColumns.CreatedAt),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list team memberships: %w", err)
	}

	memberships := make([]entity.TeamMembership, 0, len(models))

	for _, model := range models {
		membership, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		memberships = append(memberships, membership)
	}

	return memberships, nil
}

func (r *teamMemberRepository) Delete(ctx context.Context, teamID, accountID uuid.UUID) error {
	deleted, err := dbpostgres.WorkspaceTeamMembers(
		dbpostgres.WorkspaceTeamMemberWhere.TeamID.EQ(teamID.String()),
		dbpostgres.WorkspaceTeamMemberWhere.AccountID.EQ(accountID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete team membership: %w", err)
	}

	if deleted == 0 {
		return entity.ErrTeamMembershipNotFound
	}

	return nil
}
