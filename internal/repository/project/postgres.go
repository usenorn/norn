package project

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const (
	uniqueViolationCode = "23505"
	slugUniqueIndex     = "workspace_projects_slug_key"
	archiveCheck        = "workspace_projects_archive_check"
	checkViolationCode  = "23514"
)

const projectColumns = `
       p.id,
       p.workspace_id,
       p.slug,
       p.name,
       p.description,
       p.state,
       coalesce(p.lead_account_id::text, ''),
       coalesce(a.display_name, ''),
       coalesce(to_char(p.target_on, 'YYYY-MM-DD'), ''),
       p.archived_at,
       coalesce(u.health, ''),
       p.created_at,
       p.updated_at`

const projectJoins = `
FROM workspace_projects p
LEFT JOIN accounts a ON a.id = p.lead_account_id
LEFT JOIN LATERAL (
    SELECT s.health
    FROM workspace_project_status_updates s
    WHERE s.project_id = p.id
    ORDER BY s.created_at DESC
    LIMIT 1
) u ON TRUE`

const insertProjectQuery = `
WITH inserted AS (
    INSERT INTO workspace_projects (workspace_id, slug, name, description, state, lead_account_id,
                                    target_on, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6::uuid, $7::date, $8, $9)
    RETURNING id, workspace_id, slug, name, description, state, lead_account_id,
              target_on, archived_at, created_at, updated_at
)
SELECT` + projectColumns + `
FROM inserted p
LEFT JOIN accounts a ON a.id = p.lead_account_id
LEFT JOIN LATERAL (SELECT NULL::text AS health) u ON TRUE`

const projectByIDQuery = `
SELECT` + projectColumns + projectJoins + `
WHERE p.id = $1 AND p.workspace_id = $2`

const projectBySlugQuery = `
SELECT` + projectColumns + projectJoins + `
WHERE p.workspace_id = $1 AND p.slug = $2`

const lockProjectQuery = `
SELECT` + projectColumns + projectJoins + `
WHERE p.id = $1
FOR UPDATE OF p`

const projectsQuery = `
SELECT` + projectColumns + projectJoins + `
WHERE p.workspace_id = $1
  AND ($2::boolean IS TRUE OR p.archived_at IS NULL)
  AND ($3::boolean IS NOT TRUE OR p.state = $4)
  AND ($5::boolean IS NOT TRUE OR EXISTS (
        SELECT 1 FROM workspace_project_members m
        WHERE m.project_id = p.id AND m.account_id = $6::uuid
      ) OR p.lead_account_id = $6::uuid)
ORDER BY lower(p.name), p.id`

const updateProjectQuery = `
WITH updated AS (
    UPDATE workspace_projects
    SET name            = coalesce($2, name),
        description     = coalesce($3, description),
        lead_account_id = CASE WHEN $4::boolean THEN NULL
                               ELSE coalesce($5::uuid, lead_account_id) END,
        target_on       = CASE WHEN $6::boolean THEN NULL
                               ELSE coalesce($7::date, target_on) END,
        updated_at      = $8
    WHERE id = $1
    RETURNING id, workspace_id, slug, name, description, state, lead_account_id,
              target_on, archived_at, created_at, updated_at
)
SELECT` + projectColumns + `
FROM updated p
LEFT JOIN accounts a ON a.id = p.lead_account_id
LEFT JOIN LATERAL (
    SELECT s.health FROM workspace_project_status_updates s
    WHERE s.project_id = p.id ORDER BY s.created_at DESC LIMIT 1
) u ON TRUE`

const setProjectStateQuery = `
WITH updated AS (
    UPDATE workspace_projects
    SET state = $2, updated_at = $3
    WHERE id = $1
    RETURNING id, workspace_id, slug, name, description, state, lead_account_id,
              target_on, archived_at, created_at, updated_at
)
SELECT` + projectColumns + `
FROM updated p
LEFT JOIN accounts a ON a.id = p.lead_account_id
LEFT JOIN LATERAL (
    SELECT s.health FROM workspace_project_status_updates s
    WHERE s.project_id = p.id ORDER BY s.created_at DESC LIMIT 1
) u ON TRUE`

const archiveProjectQuery = `
WITH updated AS (
    UPDATE workspace_projects
    SET archived_at = $2, updated_at = $2
    WHERE id = $1 AND archived_at IS NULL
    RETURNING id, workspace_id, slug, name, description, state, lead_account_id,
              target_on, archived_at, created_at, updated_at
)
SELECT` + projectColumns + `
FROM updated p
LEFT JOIN accounts a ON a.id = p.lead_account_id
LEFT JOIN LATERAL (
    SELECT s.health FROM workspace_project_status_updates s
    WHERE s.project_id = p.id ORDER BY s.created_at DESC LIMIT 1
) u ON TRUE`

const unarchiveProjectQuery = `
WITH updated AS (
    UPDATE workspace_projects
    SET archived_at = NULL, updated_at = $2
    WHERE id = $1 AND archived_at IS NOT NULL
    RETURNING id, workspace_id, slug, name, description, state, lead_account_id,
              target_on, archived_at, created_at, updated_at
)
SELECT` + projectColumns + `
FROM updated p
LEFT JOIN accounts a ON a.id = p.lead_account_id
LEFT JOIN LATERAL (
    SELECT s.health FROM workspace_project_status_updates s
    WHERE s.project_id = p.id ORDER BY s.created_at DESC LIMIT 1
) u ON TRUE`

const deleteProjectQuery = `DELETE FROM workspace_projects WHERE id = $1`

const concealedWorkQuery = `
SELECT EXISTS (
    SELECT 1
    FROM workspace_issues i
    WHERE i.project_id = $1
      AND i.status = 'active'
      AND NOT ($2::boolean IS TRUE OR i.team_id = ANY($3::uuid[]))
)`

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch {
	case pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == slugUniqueIndex:
		return entity.ErrProjectSlugTaken
	case pgErr.Code == checkViolationCode && pgErr.ConstraintName == archiveCheck:
		return entity.ErrProjectNotFinished
	}

	return err
}

type projectRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Project {
	return &projectRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func teamIDs(scope entity.TeamScope) []string {
	ids := make([]string, 0, len(scope.TeamIDs))

	for _, id := range scope.TeamIDs {
		ids = append(ids, id.String())
	}

	return ids
}

func scanProject(row scanner) (entity.Project, error) {
	var (
		project   entity.Project
		id        string
		workspace string
		lead      string
		state     string
		health    string
	)

	if err := row.Scan(
		&id,
		&workspace,
		&project.Slug,
		&project.Name,
		&project.Description,
		&state,
		&lead,
		&project.LeadName,
		&project.TargetOn,
		&project.ArchivedAt,
		&health,
		&project.CreatedAt,
		&project.UpdatedAt,
	); err != nil {
		return entity.Project{}, err
	}

	project.State = entity.ProjectState(state)
	project.Health = entity.ProjectHealth(health)

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.Project{}, fmt.Errorf("parse project id: %w", err)
	}

	project.ID = parsed

	if project.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.Project{}, fmt.Errorf("parse project workspace id: %w", err)
	}

	if lead != "" {
		if project.LeadAccountID, err = uuid.Parse(lead); err != nil {
			return entity.Project{}, fmt.Errorf("parse project lead id: %w", err)
		}
	}

	return project, nil
}

func (r *projectRepository) Create(
	ctx context.Context,
	project entity.Project,
) (entity.Project, error) {
	var (
		lead   any
		target any
	)

	if project.LeadAccountID != uuid.Nil {
		lead = project.LeadAccountID.String()
	}

	if project.TargetOn != "" {
		target = project.TargetOn
	}

	state := project.State
	if state == "" {
		state = entity.ProjectStatePlanned
	}

	createdAt, updatedAt := entity.OriginStamp(project.Origin, time.Now().UTC())

	created, err := scanProject(r.db.Querier(ctx).QueryRowContext(
		ctx,
		insertProjectQuery,
		project.WorkspaceID.String(),
		project.Slug,
		project.Name,
		project.Description,
		string(state),
		lead,
		target,
		createdAt,
		updatedAt,
	))
	if err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.Project{}, translated
		}

		return entity.Project{}, fmt.Errorf("insert project: %w", err)
	}

	return created, nil
}

func (r *projectRepository) find(
	ctx context.Context,
	query string,
	args ...any,
) (entity.Project, error) {
	project, err := scanProject(r.db.Querier(ctx).QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Project{}, entity.ErrProjectNotFound
		}

		return entity.Project{}, fmt.Errorf("find project: %w", err)
	}

	return project, nil
}

func (r *projectRepository) GetByID(
	ctx context.Context,
	workspaceID, projectID uuid.UUID,
) (entity.Project, error) {
	return r.find(ctx, projectByIDQuery, projectID.String(), workspaceID.String())
}

func (r *projectRepository) GetBySlug(
	ctx context.Context,
	workspaceID uuid.UUID,
	slug string,
) (entity.Project, error) {
	return r.find(ctx, projectBySlugQuery, workspaceID.String(), slug)
}

func (r *projectRepository) LockByID(
	ctx context.Context,
	projectID uuid.UUID,
) (entity.Project, error) {
	return r.find(ctx, lockProjectQuery, projectID.String())
}

func (r *projectRepository) ListByWorkspaceID(
	ctx context.Context,
	workspaceID uuid.UUID,
	filter repository.ProjectFilter,
) ([]entity.Project, error) {
	account := uuid.Nil

	if filter.ForAccountID != nil {
		account = *filter.ForAccountID
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		projectsQuery,
		workspaceID.String(),
		filter.IncludeArchived,
		filter.State != "",
		string(filter.State),
		filter.ForAccountID != nil,
		account.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	defer func() { _ = rows.Close() }()

	projects := make([]entity.Project, 0)

	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, nil
}

func (r *projectRepository) UpdateSettings(
	ctx context.Context,
	projectID uuid.UUID,
	settings repository.ProjectSettings,
) (entity.Project, error) {
	var (
		name        any
		description any
		lead        any
		target      any
	)

	if settings.Name != "" {
		name = settings.Name
	}

	description = settings.Description

	if settings.LeadAccountID != nil {
		lead = settings.LeadAccountID.String()
	}

	if settings.TargetOn != "" {
		target = settings.TargetOn
	}

	updated, err := scanProject(r.db.Querier(ctx).QueryRowContext(
		ctx,
		updateProjectQuery,
		projectID.String(),
		name,
		description,
		settings.ClearLead,
		lead,
		settings.ClearTarget,
		target,
		time.Now().UTC(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Project{}, entity.ErrProjectNotFound
		}

		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.Project{}, translated
		}

		return entity.Project{}, fmt.Errorf("update project: %w", err)
	}

	return updated, nil
}

func (r *projectRepository) SetState(
	ctx context.Context,
	projectID uuid.UUID,
	state entity.ProjectState,
) (entity.Project, error) {
	updated, err := scanProject(r.db.Querier(ctx).QueryRowContext(
		ctx,
		setProjectStateQuery,
		projectID.String(),
		string(state),
		time.Now().UTC(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Project{}, entity.ErrProjectNotFound
		}

		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.Project{}, translated
		}

		return entity.Project{}, fmt.Errorf("set project state: %w", err)
	}

	return updated, nil
}

func (r *projectRepository) Archive(
	ctx context.Context,
	projectID uuid.UUID,
	archivedAt time.Time,
) (entity.Project, error) {
	archived, err := scanProject(r.db.Querier(ctx).QueryRowContext(
		ctx,
		archiveProjectQuery,
		projectID.String(),
		archivedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Project{}, entity.ErrProjectArchived
		}

		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.Project{}, translated
		}

		return entity.Project{}, fmt.Errorf("archive project: %w", err)
	}

	return archived, nil
}

func (r *projectRepository) Unarchive(
	ctx context.Context,
	projectID uuid.UUID,
) (entity.Project, error) {
	restored, err := scanProject(r.db.Querier(ctx).QueryRowContext(
		ctx,
		unarchiveProjectQuery,
		projectID.String(),
		time.Now().UTC(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Project{}, entity.ErrProjectNotArchived
		}

		return entity.Project{}, fmt.Errorf("unarchive project: %w", err)
	}

	return restored, nil
}

func (r *projectRepository) Delete(ctx context.Context, projectID uuid.UUID) error {
	result, err := r.db.Querier(ctx).ExecContext(ctx, deleteProjectQuery, projectID.String())
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	removed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read removed project rows: %w", err)
	}

	if removed == 0 {
		return entity.ErrProjectNotFound
	}

	return nil
}

func (r *projectRepository) HasConcealedWork(
	ctx context.Context,
	scope entity.TeamScope,
	projectID uuid.UUID,
) (bool, error) {
	var concealed bool

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		concealedWorkQuery,
		projectID.String(),
		scope.AllTeams,
		teamIDs(scope),
	).Scan(&concealed); err != nil {
		return false, fmt.Errorf("check concealed project work: %w", err)
	}

	return concealed, nil
}
