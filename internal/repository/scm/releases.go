package scm

import (
	"context"
	"fmt"

	"github.com/aarondl/sqlboiler/v4/types"
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

type releaseRepository struct {
	db *postgres.Client
}

func NewSCMRelease(db *postgres.Client) repository.SCMRelease {
	return &releaseRepository{db: db}
}

const releaseColumns = `
    r.id, r.repository_id, r.workspace_id, r.external_id, r.tag, r.name, r.url, r.commit_sha,
    r.prerelease, r.published_at, r.created_at, r.updated_at`

func scanRelease(row interface{ Scan(...any) error }) (entity.SCMRelease, error) {
	var release entity.SCMRelease

	err := row.Scan(
		&release.ID,
		&release.RepositoryID,
		&release.WorkspaceID,
		&release.ExternalID,
		&release.Tag,
		&release.Name,
		&release.URL,
		&release.CommitSHA,
		&release.Prerelease,
		&release.PublishedAt,
		&release.CreatedAt,
		&release.UpdatedAt,
	)
	if err != nil {
		return entity.SCMRelease{}, err
	}

	return release, nil
}

const upsertReleaseQuery = `
INSERT INTO workspace_scm_releases (
    id, repository_id, workspace_id, external_id, tag, name, url, commit_sha, prerelease,
    published_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (repository_id, external_id) DO UPDATE
SET tag = EXCLUDED.tag,
    name = EXCLUDED.name,
    url = EXCLUDED.url,
    commit_sha = EXCLUDED.commit_sha,
    prerelease = EXCLUDED.prerelease,
    published_at = EXCLUDED.published_at,
    updated_at = now()
RETURNING id, repository_id, workspace_id, external_id, tag, name, url, commit_sha,
          prerelease, published_at, created_at, updated_at`

func (r *releaseRepository) Upsert(
	ctx context.Context,
	release entity.SCMRelease,
) (entity.SCMRelease, error) {
	if release.ID == uuid.Nil {
		release.ID = uuid.New()
	}

	stored, err := scanRelease(r.db.Querier(ctx).QueryRowContext(
		ctx,
		upsertReleaseQuery,
		release.ID,
		release.RepositoryID,
		release.WorkspaceID,
		release.ExternalID,
		release.Tag,
		release.Name,
		release.URL,
		release.CommitSHA,
		release.Prerelease,
		release.PublishedAt,
	))
	if err != nil {
		return entity.SCMRelease{}, fmt.Errorf("record a release: %w", err)
	}

	return stored, nil
}

const listReleasesQuery = `
SELECT` + releaseColumns + `
FROM workspace_scm_releases AS r
WHERE r.repository_id = $1
ORDER BY r.published_at DESC NULLS LAST, r.id
LIMIT $2`

func (r *releaseRepository) ListByRepository(
	ctx context.Context,
	repositoryID uuid.UUID,
	limit int,
) (entity.SCMReleases, error) {
	return r.collect(ctx, listReleasesQuery, repositoryID, limit)
}

const listReleasesByIssueQuery = `
SELECT DISTINCT` + releaseColumns + `
FROM workspace_scm_releases AS r
JOIN workspace_scm_release_links AS l ON l.release_id = r.id
WHERE r.workspace_id = $1 AND l.issue_id = $2
ORDER BY r.published_at DESC NULLS LAST, r.id`

func (r *releaseRepository) ListByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.SCMReleases, error) {
	return r.collect(ctx, listReleasesByIssueQuery, workspaceID, issueID)
}

func (r *releaseRepository) collect(
	ctx context.Context,
	query string,
	args ...any,
) (entity.SCMReleases, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}

	defer func() { _ = rows.Close() }()

	releases := make(entity.SCMReleases, 0)

	for rows.Next() {
		release, err := scanRelease(rows)
		if err != nil {
			return nil, fmt.Errorf("read a release: %w", err)
		}

		releases = append(releases, release)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}

	return releases, nil
}

const linkReleaseChangeQuery = `
INSERT INTO workspace_scm_release_links (release_id, link_id, issue_id)
VALUES ($1, $2, $3)
ON CONFLICT (release_id, link_id) DO NOTHING`

func (r *releaseRepository) LinkChanges(
	ctx context.Context,
	releaseID uuid.UUID,
	links []entity.CodeLink,
) error {
	querier := r.db.Querier(ctx)

	for _, link := range links {
		if _, err := querier.ExecContext(
			ctx, linkReleaseChangeQuery, releaseID, link.ID, link.IssueID,
		); err != nil {
			return fmt.Errorf("record what a release shipped: %w", err)
		}
	}

	return nil
}

type deploymentRepository struct {
	db *postgres.Client
}

func NewSCMDeployment(db *postgres.Client) repository.SCMDeployment {
	return &deploymentRepository{db: db}
}

const upsertDeploymentQuery = `
INSERT INTO workspace_scm_deployments (
    id, repository_id, workspace_id, external_id, environment, state, url, commit_sha,
    occurred_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (repository_id, external_id) DO UPDATE
SET environment = EXCLUDED.environment,
    state = EXCLUDED.state,
    url = EXCLUDED.url,
    commit_sha = EXCLUDED.commit_sha,
    occurred_at = EXCLUDED.occurred_at,
    updated_at = now()`

func (r *deploymentRepository) Upsert(
	ctx context.Context,
	deployment entity.SCMDeployment,
) error {
	if deployment.ID == uuid.Nil {
		deployment.ID = uuid.New()
	}

	_, err := r.db.Querier(ctx).ExecContext(
		ctx,
		upsertDeploymentQuery,
		deployment.ID,
		deployment.RepositoryID,
		deployment.WorkspaceID,
		deployment.ExternalID,
		deployment.Environment,
		deployment.State,
		deployment.URL,
		deployment.CommitSHA,
		deployment.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("record a deployment: %w", err)
	}

	return nil
}

const listDeploymentsQuery = `
SELECT id, repository_id, workspace_id, external_id, environment, state, url, commit_sha,
       occurred_at, created_at, updated_at
FROM workspace_scm_deployments
WHERE repository_id = $1 AND commit_sha = ANY($2::text[])
ORDER BY occurred_at DESC NULLS LAST, id`

func (r *deploymentRepository) ListByCommits(
	ctx context.Context,
	repositoryID uuid.UUID,
	commits []string,
) (entity.SCMDeployments, error) {
	found := make(entity.SCMDeployments, 0)

	if len(commits) == 0 {
		return found, nil
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listDeploymentsQuery, repositoryID, types.StringArray(commits),
	)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var deployment entity.SCMDeployment

		if err := rows.Scan(
			&deployment.ID,
			&deployment.RepositoryID,
			&deployment.WorkspaceID,
			&deployment.ExternalID,
			&deployment.Environment,
			&deployment.State,
			&deployment.URL,
			&deployment.CommitSHA,
			&deployment.OccurredAt,
			&deployment.CreatedAt,
			&deployment.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("read a deployment: %w", err)
		}

		found = append(found, deployment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	return found, nil
}
