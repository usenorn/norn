package team

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
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
	uniqueViolationCode = "23505"
	keyUniqueIndex      = "workspace_teams_workspace_key_key"
)

func toEntity(model *dbpostgres.WorkspaceTeam) (entity.Team, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.Team{}, fmt.Errorf("parse team id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.Team{}, fmt.Errorf("parse team workspace id: %w", err)
	}

	team := entity.Team{
		ID:          id,
		WorkspaceID: workspaceID,
		Key:         model.Key,
		Name:        model.Name,
		Status:      entity.TeamStatus(model.Status),
		Visibility:  entity.TeamVisibility(model.Visibility),
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}

	if model.ArchivedAt.Valid {
		archivedAt := model.ArchivedAt.Time
		team.ArchivedAt = &archivedAt
	}

	return team, nil
}

func toModel(team entity.Team) *dbpostgres.WorkspaceTeam {
	model := &dbpostgres.WorkspaceTeam{
		ID:          team.ID.String(),
		WorkspaceID: team.WorkspaceID.String(),
		Key:         team.Key,
		Name:        team.Name,
		Status:      string(team.Status),
		Visibility:  string(team.Visibility),
		CreatedAt:   team.CreatedAt,
		UpdatedAt:   team.UpdatedAt,
	}

	if team.ArchivedAt != nil {
		model.ArchivedAt = null.TimeFrom(*team.ArchivedAt)
	}

	return model
}

type teamRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Team {
	return &teamRepository{db: db}
}

func (r *teamRepository) Create(ctx context.Context, team entity.Team) (entity.Team, error) {
	if team.ID == uuid.Nil {
		team.ID = uuid.New()
	}

	now := time.Now().UTC()
	team.CreatedAt = now
	team.UpdatedAt = now

	model := toModel(team)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == keyUniqueIndex {
			return entity.Team{}, entity.ErrTeamKeyTaken
		}

		return entity.Team{}, fmt.Errorf("insert team: %w", err)
	}

	return toEntity(model)
}

func (r *teamRepository) GetByID(ctx context.Context, id uuid.UUID) (entity.Team, error) {
	model, err := dbpostgres.FindWorkspaceTeam(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Team{}, entity.ErrTeamNotFound
		}

		return entity.Team{}, fmt.Errorf("find team by id: %w", err)
	}

	return toEntity(model)
}

func (r *teamRepository) ListVisibleTo(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	status entity.TeamStatus,
	includePrivate bool,
) ([]entity.Team, error) {
	mods := []qm.QueryMod{
		dbpostgres.WorkspaceTeamWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.OrderBy(dbpostgres.WorkspaceTeamColumns.Key),
	}

	if status != "" {
		mods = append(mods, dbpostgres.WorkspaceTeamWhere.Status.EQ(string(status)))
	}

	if !includePrivate {
		mods = append(mods, qm.Where(
			"(workspace_teams.visibility = ? OR EXISTS ("+
				"SELECT 1 FROM workspace_team_members m WHERE m.team_id = workspace_teams.id AND m.account_id = ?))",
			string(entity.TeamVisibilityPublic),
			accountID.String(),
		))
	}

	models, err := dbpostgres.WorkspaceTeams(mods...).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list teams visible to account: %w", err)
	}

	teams := make([]entity.Team, 0, len(models))

	for _, model := range models {
		team, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		teams = append(teams, team)
	}

	return teams, nil
}

func (r *teamRepository) ListByWorkspaceMember(ctx context.Context, workspaceID, accountID uuid.UUID) ([]entity.Team, error) {
	models, err := dbpostgres.WorkspaceTeams(
		dbpostgres.WorkspaceTeamWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.Where(
			"EXISTS (SELECT 1 FROM workspace_team_members m WHERE m.team_id = workspace_teams.id AND m.account_id = ?)",
			accountID.String(),
		),
		qm.OrderBy(dbpostgres.WorkspaceTeamColumns.Key),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list teams by workspace member: %w", err)
	}

	teams := make([]entity.Team, 0, len(models))

	for _, model := range models {
		team, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		teams = append(teams, team)
	}

	return teams, nil
}

func (r *teamRepository) UpdateSettings(
	ctx context.Context,
	id uuid.UUID,
	name string,
	visibility entity.TeamVisibility,
) (entity.Team, error) {
	updated, err := dbpostgres.WorkspaceTeams(
		dbpostgres.WorkspaceTeamWhere.ID.EQ(id.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceTeamColumns.Name:       name,
		dbpostgres.WorkspaceTeamColumns.Visibility: string(visibility),
		dbpostgres.WorkspaceTeamColumns.UpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		return entity.Team{}, fmt.Errorf("update team settings: %w", err)
	}

	if updated == 0 {
		return entity.Team{}, entity.ErrTeamNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *teamRepository) Archive(ctx context.Context, id uuid.UUID, archivedAt time.Time) (entity.Team, error) {
	updated, err := dbpostgres.WorkspaceTeams(
		dbpostgres.WorkspaceTeamWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceTeamWhere.Status.EQ(string(entity.TeamStatusActive)),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceTeamColumns.Status:     string(entity.TeamStatusArchived),
		dbpostgres.WorkspaceTeamColumns.ArchivedAt: null.TimeFrom(archivedAt),
		dbpostgres.WorkspaceTeamColumns.UpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		return entity.Team{}, fmt.Errorf("archive team: %w", err)
	}

	if updated == 0 {
		return entity.Team{}, entity.ErrTeamArchived
	}

	return r.GetByID(ctx, id)
}

func (r *teamRepository) Unarchive(ctx context.Context, id uuid.UUID) (entity.Team, error) {
	updated, err := dbpostgres.WorkspaceTeams(
		dbpostgres.WorkspaceTeamWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceTeamWhere.Status.EQ(string(entity.TeamStatusArchived)),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceTeamColumns.Status:     string(entity.TeamStatusActive),
		dbpostgres.WorkspaceTeamColumns.ArchivedAt: null.NewTime(time.Time{}, false),
		dbpostgres.WorkspaceTeamColumns.UpdatedAt:  time.Now().UTC(),
	})
	if err != nil {
		return entity.Team{}, fmt.Errorf("unarchive team: %w", err)
	}

	if updated == 0 {
		return entity.Team{}, entity.ErrTeamNotArchived
	}

	return r.GetByID(ctx, id)
}
