package executionservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const serviceColumns = `
       id,
       execution_id,
       workspace_id,
       name,
       state,
       probe,
       port,
       reason,
       reported_at,
       created_at,
       updated_at`

const saveServiceQuery = `
WITH upserted AS (
    INSERT INTO workspace_execution_services
        (execution_id, workspace_id, name, state, probe, port, reason, reported_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    ON CONFLICT (execution_id, name) DO UPDATE
    SET state       = excluded.state,
        probe       = CASE WHEN excluded.probe <> ''
                           THEN excluded.probe
                           ELSE workspace_execution_services.probe END,
        port        = CASE WHEN excluded.port > 0
                           THEN excluded.port
                           ELSE workspace_execution_services.port END,
        reason      = excluded.reason,
        reported_at = excluded.reported_at,
        updated_at  = now()
    WHERE excluded.reported_at >= workspace_execution_services.reported_at
    RETURNING *
)
SELECT` + serviceColumns + `
FROM upserted`

const serviceCountQuery = `
SELECT count(*)
FROM workspace_execution_services
WHERE execution_id = $1`

type serviceRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ExecutionService {
	return &serviceRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func toEntity(model *dbpostgres.WorkspaceExecutionService) (entity.ExecutionService, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.ExecutionService{}, fmt.Errorf("parse service id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.ExecutionService{}, fmt.Errorf("parse workspace id: %w", err)
	}

	return entity.ExecutionService{
		ID:          id,
		ExecutionID: model.ExecutionID,
		WorkspaceID: workspaceID,
		Name:        model.Name,
		State:       entity.ExecutionServiceState(model.State),
		Probe:       entity.ExecutionServiceProbe(model.Probe),
		Port:        model.Port,
		Reason:      model.Reason,
		ReportedAt:  model.ReportedAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func toEntities(
	models dbpostgres.WorkspaceExecutionServiceSlice,
) ([]entity.ExecutionService, error) {
	services := make([]entity.ExecutionService, 0, len(models))

	for _, model := range models {
		service, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		services = append(services, service)
	}

	return services, nil
}

func (r *serviceRepository) Save(
	ctx context.Context,
	service entity.ExecutionService,
) (entity.ExecutionService, error) {
	saved, err := scanService(r.db.Querier(ctx).QueryRowContext(
		ctx,
		saveServiceQuery,
		service.ExecutionID,
		service.WorkspaceID.String(),
		service.Name,
		string(service.State),
		string(service.Probe),
		service.Port,
		service.Reason,
		service.ReportedAt,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.ExecutionService{}, entity.ErrExecutionServiceStale
	}

	if err != nil {
		return entity.ExecutionService{}, fmt.Errorf("save the state of a run's service: %w", err)
	}

	return saved, nil
}

func (r *serviceRepository) ByExecution(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionService, error) {
	models, err := dbpostgres.WorkspaceExecutionServices(
		dbpostgres.WorkspaceExecutionServiceWhere.ExecutionID.EQ(executionID),
		qm.OrderBy(
			dbpostgres.WorkspaceExecutionServiceColumns.Name+", "+
				dbpostgres.WorkspaceExecutionServiceColumns.ID,
		),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list the services of a run: %w", err)
	}

	return toEntities(models)
}

func (r *serviceRepository) Count(ctx context.Context, executionID string) (int, error) {
	var held int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, serviceCountQuery, executionID,
	).Scan(&held); err != nil {
		return 0, fmt.Errorf("count the services of a run: %w", err)
	}

	return held, nil
}

func scanService(row scanner) (entity.ExecutionService, error) {
	var (
		service     entity.ExecutionService
		id          string
		workspaceID string
		state       string
		probe       string
	)

	if err := row.Scan(
		&id,
		&service.ExecutionID,
		&workspaceID,
		&service.Name,
		&state,
		&probe,
		&service.Port,
		&service.Reason,
		&service.ReportedAt,
		&service.CreatedAt,
		&service.UpdatedAt,
	); err != nil {
		return entity.ExecutionService{}, err
	}

	parsedID, err := uuid.Parse(id)
	if err != nil {
		return entity.ExecutionService{}, fmt.Errorf("parse service id: %w", err)
	}

	parsedWorkspace, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.ExecutionService{}, fmt.Errorf("parse workspace id: %w", err)
	}

	service.ID = parsedID
	service.WorkspaceID = parsedWorkspace
	service.State = entity.ExecutionServiceState(state)
	service.Probe = entity.ExecutionServiceProbe(probe)

	return service, nil
}
