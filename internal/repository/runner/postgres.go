package runner

import (
	"context"
	"crypto/ed25519"
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
	nameUniqueIndex     = "workspace_runners_live_name_key"
)

func toEntity(model *dbpostgres.WorkspaceRunner) (entity.Runner, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.Runner{}, fmt.Errorf("parse runner id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.Runner{}, fmt.Errorf("parse runner workspace id: %w", err)
	}

	agentID, err := uuid.Parse(model.AgentID)
	if err != nil {
		return entity.Runner{}, fmt.Errorf("parse runner agent id: %w", err)
	}

	runner := entity.Runner{
		ID:          id,
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		Name:        model.Name,
		Host: entity.RunnerHost{
			Hostname: model.Hostname,
			OS:       model.Os,
			Arch:     model.Arch,
			Version:  model.RunnerVersion,
		},
		Authority:   entity.NewRequestedAuthority(model.AllTeams, model.TeamIds, model.Scopes),
		PublicKey:   ed25519.PublicKey(model.PublicKey),
		RefreshHash: model.RefreshHash,
		Status:      entity.RunnerStatus(model.Status),
		EnrolledAt:  model.EnrolledAt,
		UpdatedAt:   model.UpdatedAt,
	}

	if model.LastSeenAt.Valid {
		seen := model.LastSeenAt.Time
		runner.LastSeenAt = &seen
	}

	if model.RevokedAt.Valid {
		revoked := model.RevokedAt.Time
		runner.RevokedAt = &revoked
	}

	return runner, nil
}

func toModel(runner entity.Runner) *dbpostgres.WorkspaceRunner {
	model := &dbpostgres.WorkspaceRunner{
		ID:            runner.ID.String(),
		WorkspaceID:   runner.WorkspaceID.String(),
		AgentID:       runner.AgentID.String(),
		Name:          runner.Name,
		Hostname:      runner.Host.Hostname,
		Os:            runner.Host.OS,
		Arch:          runner.Host.Arch,
		RunnerVersion: runner.Host.Version,
		AllTeams:      runner.Authority.AllTeams,
		TeamIds:       runner.Authority.TeamStrings(),
		Scopes:        runner.Authority.ScopeStrings(),
		PublicKey:     runner.PublicKey,
		RefreshHash:   runner.RefreshHash,
		Status:        string(runner.Status),
		EnrolledAt:    runner.EnrolledAt,
		UpdatedAt:     runner.UpdatedAt,
	}

	if runner.LastSeenAt != nil {
		model.LastSeenAt = null.TimeFrom(*runner.LastSeenAt)
	}

	if runner.RevokedAt != nil {
		model.RevokedAt = null.TimeFrom(*runner.RevokedAt)
	}

	return model
}

func toEntities(models dbpostgres.WorkspaceRunnerSlice) ([]entity.Runner, error) {
	runners := make([]entity.Runner, 0, len(models))

	for _, model := range models {
		runner, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		runners = append(runners, runner)
	}

	return runners, nil
}

type runnerRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Runner {
	return &runnerRepository{db: db}
}

func (r *runnerRepository) Enrol(ctx context.Context, runner entity.Runner) (entity.Runner, error) {
	if runner.ID == uuid.Nil {
		runner.ID = uuid.New()
	}

	model := toModel(runner)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == uniqueViolationCode &&
			pgErr.ConstraintName == nameUniqueIndex {
			return entity.Runner{}, entity.ErrRunnerNameTaken
		}

		return entity.Runner{}, fmt.Errorf("insert runner: %w", err)
	}

	return toEntity(model)
}

func (r *runnerRepository) one(ctx context.Context, mods ...qm.QueryMod) (entity.Runner, error) {
	model, err := dbpostgres.WorkspaceRunners(mods...).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Runner{}, entity.ErrRunnerNotFound
		}

		return entity.Runner{}, fmt.Errorf("find runner: %w", err)
	}

	return toEntity(model)
}

func (r *runnerRepository) GetByID(ctx context.Context, runnerID uuid.UUID) (entity.Runner, error) {
	return r.one(ctx, dbpostgres.WorkspaceRunnerWhere.ID.EQ(runnerID.String()))
}

func (r *runnerRepository) GetByRefreshHash(
	ctx context.Context,
	refreshHash []byte,
) (entity.Runner, error) {
	return r.one(ctx, dbpostgres.WorkspaceRunnerWhere.RefreshHash.EQ(refreshHash))
}

func (r *runnerRepository) ListByWorkspaceID(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.Runner, error) {
	models, err := dbpostgres.WorkspaceRunners(
		dbpostgres.WorkspaceRunnerWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.OrderBy(
			dbpostgres.WorkspaceRunnerColumns.EnrolledAt+" DESC, "+
				dbpostgres.WorkspaceRunnerColumns.ID,
		),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list runners: %w", err)
	}

	return toEntities(models)
}

func (r *runnerRepository) Revoke(
	ctx context.Context,
	workspaceID, runnerID uuid.UUID,
	revokedAt time.Time,
) error {
	affected, err := dbpostgres.WorkspaceRunners(
		dbpostgres.WorkspaceRunnerWhere.ID.EQ(runnerID.String()),
		dbpostgres.WorkspaceRunnerWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceRunnerWhere.Status.EQ(string(entity.RunnerStatusActive)),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceRunnerColumns.Status:    string(entity.RunnerStatusRevoked),
		dbpostgres.WorkspaceRunnerColumns.RevokedAt: revokedAt,
		dbpostgres.WorkspaceRunnerColumns.UpdatedAt: revokedAt,
	})
	if err != nil {
		return fmt.Errorf("revoke runner: %w", err)
	}

	if affected == 0 {
		return entity.ErrRunnerNotFound
	}

	return nil
}

func (r *runnerRepository) RecordSeen(
	ctx context.Context,
	runnerID uuid.UUID,
	seenAt time.Time,
) error {
	_, err := dbpostgres.WorkspaceRunners(
		dbpostgres.WorkspaceRunnerWhere.ID.EQ(runnerID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceRunnerColumns.LastSeenAt: seenAt,
	})
	if err != nil {
		return fmt.Errorf("record runner seen: %w", err)
	}

	return nil
}
