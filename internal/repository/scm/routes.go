package scm

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

type routeRepository struct {
	db *postgres.Client
}

func NewSCMRoute(db *postgres.Client) repository.SCMRoute {
	return &routeRepository{db: db}
}

const routeColumns = `
    id, repository_id, workspace_id, team_id, path_prefix, created_at, updated_at`

func scanRoute(row interface{ Scan(...any) error }) (entity.SCMRoute, error) {
	var route entity.SCMRoute

	err := row.Scan(
		&route.ID,
		&route.RepositoryID,
		&route.WorkspaceID,
		&route.TeamID,
		&route.PathPrefix,
		&route.CreatedAt,
		&route.UpdatedAt,
	)
	if err != nil {
		return entity.SCMRoute{}, err
	}

	return route, nil
}

const insertRouteQuery = `
INSERT INTO workspace_scm_routes (id, repository_id, workspace_id, team_id, path_prefix)
VALUES ($1, $2, $3, $4, $5)
RETURNING` + routeColumns

func (r *routeRepository) Create(
	ctx context.Context,
	route entity.SCMRoute,
) (entity.SCMRoute, error) {
	if route.ID == uuid.Nil {
		route.ID = uuid.New()
	}

	created, err := scanRoute(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertRouteQuery,
		route.ID,
		route.RepositoryID,
		route.WorkspaceID,
		route.TeamID,
		route.PathPrefix,
	))
	if err != nil {
		if violates(err, routePrefixUniqueIndex) {
			return entity.SCMRoute{}, entity.ErrSCMRouteExists
		}

		return entity.SCMRoute{}, fmt.Errorf("create route: %w", err)
	}

	return created, nil
}

const listRoutesByRepositoryQuery = `
SELECT` + routeColumns + `
FROM workspace_scm_routes
WHERE repository_id = $1
ORDER BY length(path_prefix) DESC, path_prefix, team_id`

func (r *routeRepository) ListByRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
) (entity.SCMRoutes, error) {
	return r.list(ctx, listRoutesByRepositoryQuery, repositoryID)
}

const listRoutesByWorkspaceQuery = `
SELECT` + routeColumns + `
FROM workspace_scm_routes
WHERE workspace_id = $1
ORDER BY length(path_prefix) DESC, path_prefix, team_id`

func (r *routeRepository) ListByWorkspace(
	ctx context.Context,
	workspaceID uuid.UUID,
) (entity.SCMRoutes, error) {
	return r.list(ctx, listRoutesByWorkspaceQuery, workspaceID)
}

func (r *routeRepository) list(
	ctx context.Context,
	query string,
	args ...any,
) (entity.SCMRoutes, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}

	defer func() { _ = rows.Close() }()

	routes := make(entity.SCMRoutes, 0)

	for rows.Next() {
		route, err := scanRoute(rows)
		if err != nil {
			return nil, fmt.Errorf("read route: %w", err)
		}

		routes = append(routes, route)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}

	return routes, nil
}

const deleteRouteQuery = `
DELETE FROM workspace_scm_routes
WHERE workspace_id = $1 AND id = $2
RETURNING` + routeColumns

func (r *routeRepository) Delete(
	ctx context.Context,
	workspaceID, routeID uuid.UUID,
) (entity.SCMRoute, error) {
	route, err := scanRoute(
		r.db.Querier(ctx).QueryRowContext(ctx, deleteRouteQuery, workspaceID, routeID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.SCMRoute{}, entity.ErrSCMRouteNotFound
	}

	if err != nil {
		return entity.SCMRoute{}, fmt.Errorf("delete route: %w", err)
	}

	return route, nil
}
