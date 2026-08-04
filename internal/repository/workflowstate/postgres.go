package workflowstate

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
	uniqueViolationCode = "23505"
	nameUniqueIndex     = "workspace_workflow_states_team_name_key"
)

const repositionQuery = `
UPDATE workspace_workflow_states AS s
SET position = ordered.position, updated_at = $3
FROM (
    SELECT id, ordinality AS position
    FROM unnest($2::uuid[]) WITH ORDINALITY AS t (id, ordinality)
) AS ordered
WHERE s.id = ordered.id AND s.team_id = $1`

func toEntity(model *dbpostgres.WorkspaceWorkflowState) (entity.WorkflowState, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.WorkflowState{}, fmt.Errorf("parse workflow state id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.WorkflowState{}, fmt.Errorf("parse workflow state workspace id: %w", err)
	}

	teamID, err := uuid.Parse(model.TeamID)
	if err != nil {
		return entity.WorkflowState{}, fmt.Errorf("parse workflow state team id: %w", err)
	}

	return entity.WorkflowState{
		ID:           id,
		WorkspaceID:  workspaceID,
		TeamID:       teamID,
		Name:         model.Name,
		Category:     entity.StateCategory(model.Category),
		Position:     model.Position,
		IsDefault:    model.IsDefault,
		IsCompletion: model.IsCompletion,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}, nil
}

func toModel(state entity.WorkflowState) *dbpostgres.WorkspaceWorkflowState {
	return &dbpostgres.WorkspaceWorkflowState{
		ID:           state.ID.String(),
		WorkspaceID:  state.WorkspaceID.String(),
		TeamID:       state.TeamID.String(),
		Name:         state.Name,
		Category:     string(state.Category),
		Position:     state.Position,
		IsDefault:    state.IsDefault,
		IsCompletion: state.IsCompletion,
		CreatedAt:    state.CreatedAt,
		UpdatedAt:    state.UpdatedAt,
	}
}

func toEntities(models dbpostgres.WorkspaceWorkflowStateSlice) ([]entity.WorkflowState, error) {
	states := make([]entity.WorkflowState, 0, len(models))

	for _, model := range models {
		state, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		states = append(states, state)
	}

	return states, nil
}

type workflowStateRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.WorkflowState {
	return &workflowStateRepository{db: db}
}

func (r *workflowStateRepository) insert(ctx context.Context, state entity.WorkflowState) (entity.WorkflowState, error) {
	if state.ID == uuid.Nil {
		state.ID = uuid.New()
	}

	now := time.Now().UTC()
	state.CreatedAt = now
	state.UpdatedAt = now

	model := toModel(state)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == nameUniqueIndex {
			return entity.WorkflowState{}, entity.ErrWorkflowStateNameTaken
		}

		return entity.WorkflowState{}, fmt.Errorf("insert workflow state: %w", err)
	}

	return toEntity(model)
}

func (r *workflowStateRepository) Create(ctx context.Context, state entity.WorkflowState) (entity.WorkflowState, error) {
	return r.insert(ctx, state)
}

func (r *workflowStateRepository) CreateMany(
	ctx context.Context,
	states []entity.WorkflowState,
) ([]entity.WorkflowState, error) {
	created := make([]entity.WorkflowState, 0, len(states))

	for _, state := range states {
		inserted, err := r.insert(ctx, state)
		if err != nil {
			return nil, err
		}

		created = append(created, inserted)
	}

	return created, nil
}

func (r *workflowStateRepository) ordered(teamID uuid.UUID) []qm.QueryMod {
	return []qm.QueryMod{
		dbpostgres.WorkspaceWorkflowStateWhere.TeamID.EQ(teamID.String()),
		qm.OrderBy(
			dbpostgres.WorkspaceWorkflowStateColumns.Position + ", " +
				dbpostgres.WorkspaceWorkflowStateColumns.ID,
		),
	}
}

func (r *workflowStateRepository) ListByTeamID(
	ctx context.Context,
	teamID uuid.UUID,
) ([]entity.WorkflowState, error) {
	models, err := dbpostgres.WorkspaceWorkflowStates(r.ordered(teamID)...).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list workflow states: %w", err)
	}

	return toEntities(models)
}

func (r *workflowStateRepository) LockByTeamID(
	ctx context.Context,
	teamID uuid.UUID,
) ([]entity.WorkflowState, error) {
	models, err := dbpostgres.WorkspaceWorkflowStates(
		append(r.ordered(teamID), qm.For("UPDATE"))...,
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("lock workflow states: %w", err)
	}

	return toEntities(models)
}

func (r *workflowStateRepository) DefaultForTeam(
	ctx context.Context,
	teamID uuid.UUID,
) (entity.WorkflowState, error) {
	model, err := dbpostgres.WorkspaceWorkflowStates(
		dbpostgres.WorkspaceWorkflowStateWhere.TeamID.EQ(teamID.String()),
		dbpostgres.WorkspaceWorkflowStateWhere.IsDefault.EQ(true),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.WorkflowState{}, entity.ErrWorkflowStateNotFound
		}

		return entity.WorkflowState{}, fmt.Errorf("find default workflow state: %w", err)
	}

	return toEntity(model)
}

func (r *workflowStateRepository) UpdateSettings(
	ctx context.Context,
	id uuid.UUID,
	name string,
	category entity.StateCategory,
) (entity.WorkflowState, error) {
	updated, err := dbpostgres.WorkspaceWorkflowStates(
		dbpostgres.WorkspaceWorkflowStateWhere.ID.EQ(id.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceWorkflowStateColumns.Name:      name,
		dbpostgres.WorkspaceWorkflowStateColumns.Category:  string(category),
		dbpostgres.WorkspaceWorkflowStateColumns.UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == nameUniqueIndex {
			return entity.WorkflowState{}, entity.ErrWorkflowStateNameTaken
		}

		return entity.WorkflowState{}, fmt.Errorf("update workflow state: %w", err)
	}

	if updated == 0 {
		return entity.WorkflowState{}, entity.ErrWorkflowStateNotFound
	}

	model, err := dbpostgres.FindWorkspaceWorkflowState(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		return entity.WorkflowState{}, fmt.Errorf("read updated workflow state: %w", err)
	}

	return toEntity(model)
}

func (r *workflowStateRepository) Reposition(
	ctx context.Context,
	teamID uuid.UUID,
	orderedIDs []uuid.UUID,
) error {
	ids := make([]string, 0, len(orderedIDs))

	for _, id := range orderedIDs {
		ids = append(ids, id.String())
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		repositionQuery,
		teamID.String(),
		ids,
		time.Now().UTC(),
	); err != nil {
		return fmt.Errorf("reposition workflow states: %w", err)
	}

	return nil
}

func (r *workflowStateRepository) setFlag(
	ctx context.Context,
	teamID, stateID uuid.UUID,
	column string,
) error {
	now := time.Now().UTC()

	if _, err := dbpostgres.WorkspaceWorkflowStates(
		dbpostgres.WorkspaceWorkflowStateWhere.TeamID.EQ(teamID.String()),
		qm.Where(column+" = true"),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		column: false,
		dbpostgres.WorkspaceWorkflowStateColumns.UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("clear workflow state flag: %w", err)
	}

	updated, err := dbpostgres.WorkspaceWorkflowStates(
		dbpostgres.WorkspaceWorkflowStateWhere.ID.EQ(stateID.String()),
		dbpostgres.WorkspaceWorkflowStateWhere.TeamID.EQ(teamID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		column: true,
		dbpostgres.WorkspaceWorkflowStateColumns.UpdatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("set workflow state flag: %w", err)
	}

	if updated == 0 {
		return entity.ErrWorkflowStateNotFound
	}

	return nil
}

func (r *workflowStateRepository) SetDefault(ctx context.Context, teamID, stateID uuid.UUID) error {
	return r.setFlag(ctx, teamID, stateID, dbpostgres.WorkspaceWorkflowStateColumns.IsDefault)
}

func (r *workflowStateRepository) SetCompletion(ctx context.Context, teamID, stateID uuid.UUID) error {
	return r.setFlag(ctx, teamID, stateID, dbpostgres.WorkspaceWorkflowStateColumns.IsCompletion)
}

func (r *workflowStateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	deleted, err := dbpostgres.WorkspaceWorkflowStates(
		dbpostgres.WorkspaceWorkflowStateWhere.ID.EQ(id.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete workflow state: %w", err)
	}

	if deleted == 0 {
		return entity.ErrWorkflowStateNotFound
	}

	return nil
}
