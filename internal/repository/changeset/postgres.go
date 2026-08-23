package changeset

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

const resultColumns = `
       execution_id,
       workspace_id,
       summary,
       reported_at,
       created_at,
       updated_at`

const saveResultQuery = `
WITH upserted AS (
    INSERT INTO workspace_execution_results
        (execution_id, workspace_id, summary, reported_at)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (execution_id) DO UPDATE
    SET summary     = excluded.summary,
        reported_at = excluded.reported_at,
        updated_at  = now()
    WHERE excluded.reported_at >= workspace_execution_results.reported_at
    RETURNING *
)
SELECT` + resultColumns + `
FROM upserted`

const resultQuery = `
SELECT` + resultColumns + `
FROM workspace_execution_results
WHERE execution_id = $1`

const changeColumns = `
       id,
       execution_id,
       workspace_id,
       repository,
       branch,
       base_sha,
       head_sha,
       commits,
       additions,
       deletions,
       files_changed,
       coalesce(diff_artifact_id::text, ''),
       pull_request_url,
       coalesce(code_link_id::text, ''),
       reported_at,
       created_at,
       updated_at`

const saveChangeQuery = `
WITH upserted AS (
    INSERT INTO workspace_execution_changes
        (execution_id, workspace_id, repository, branch, base_sha, head_sha, commits,
         additions, deletions, files_changed, diff_artifact_id, pull_request_url, reported_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, nullif($11, '')::uuid, $12, $13)
    ON CONFLICT (execution_id, repository) DO UPDATE
    SET branch           = excluded.branch,
        base_sha         = excluded.base_sha,
        head_sha         = excluded.head_sha,
        commits          = excluded.commits,
        additions        = excluded.additions,
        deletions        = excluded.deletions,
        files_changed    = excluded.files_changed,
        diff_artifact_id = coalesce(excluded.diff_artifact_id,
                                    workspace_execution_changes.diff_artifact_id),
        pull_request_url = CASE WHEN excluded.pull_request_url <> ''
                                THEN excluded.pull_request_url
                                ELSE workspace_execution_changes.pull_request_url END,
        reported_at      = excluded.reported_at,
        updated_at       = now()
    WHERE excluded.reported_at >= workspace_execution_changes.reported_at
    RETURNING *
)
SELECT` + changeColumns + `
FROM upserted`

const changeByRepositoryQuery = `
SELECT` + changeColumns + `
FROM workspace_execution_changes
WHERE execution_id = $1 AND repository = $2`

const changesQuery = `
SELECT` + changeColumns + `
FROM workspace_execution_changes
WHERE execution_id = $1
ORDER BY repository, id`

const linkChangeQuery = `
UPDATE workspace_execution_changes
SET code_link_id = $2, updated_at = now()
WHERE id = $1`

const joinedChangeColumns = `
       c.id,
       c.execution_id,
       c.workspace_id,
       c.repository,
       c.branch,
       c.base_sha,
       c.head_sha,
       c.commits,
       c.additions,
       c.deletions,
       c.files_changed,
       coalesce(c.diff_artifact_id::text, ''),
       c.pull_request_url,
       coalesce(c.code_link_id::text, ''),
       c.reported_at,
       c.created_at,
       c.updated_at`

const issueChangesQuery = `
SELECT DISTINCT ON (c.repository)` + joinedChangeColumns + `,
       e.attempt
FROM workspace_execution_changes c
JOIN workspace_executions e ON e.id = c.execution_id
WHERE e.workspace_id = $1 AND e.issue_id = $2
ORDER BY c.repository, e.attempt DESC, c.reported_at DESC, c.id`

const validationColumns = `
       id,
       execution_id,
       workspace_id,
       check_name,
       status,
       detail,
       coalesce(artifact_id::text, ''),
       reported_at,
       created_at,
       updated_at`

const saveValidationQuery = `
WITH upserted AS (
    INSERT INTO workspace_execution_validations
        (execution_id, workspace_id, check_name, status, detail, artifact_id, reported_at)
    VALUES ($1, $2, $3, $4, $5, nullif($6, '')::uuid, $7)
    ON CONFLICT (execution_id, check_name) DO UPDATE
    SET status      = excluded.status,
        detail      = excluded.detail,
        artifact_id = coalesce(excluded.artifact_id,
                               workspace_execution_validations.artifact_id),
        reported_at = excluded.reported_at,
        updated_at  = now()
    WHERE excluded.reported_at >= workspace_execution_validations.reported_at
    RETURNING *
)
SELECT` + validationColumns + `
FROM upserted`

const validationByCheckQuery = `
SELECT` + validationColumns + `
FROM workspace_execution_validations
WHERE execution_id = $1 AND check_name = $2`

const validationsQuery = `
SELECT` + validationColumns + `
FROM workspace_execution_validations
WHERE execution_id = $1
ORDER BY check_name, id`

type changeSetRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ChangeSet {
	return &changeSetRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *changeSetRepository) SaveResult(
	ctx context.Context,
	result entity.ExecutionResult,
) (entity.ExecutionResult, error) {
	saved, err := scanResult(r.db.Querier(ctx).QueryRowContext(
		ctx,
		saveResultQuery,
		result.ExecutionID,
		result.WorkspaceID.String(),
		result.Summary,
		result.ReportedAt,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return r.result(ctx, result.ExecutionID)
	}

	if err != nil {
		return entity.ExecutionResult{}, fmt.Errorf("save execution result: %w", err)
	}

	return saved, nil
}

func (r *changeSetRepository) result(
	ctx context.Context,
	executionID string,
) (entity.ExecutionResult, error) {
	stored, err := scanResult(r.db.Querier(ctx).QueryRowContext(ctx, resultQuery, executionID))

	if errors.Is(err, sql.ErrNoRows) {
		return entity.ExecutionResult{}, entity.ErrExecutionResultNotFound
	}

	if err != nil {
		return entity.ExecutionResult{}, fmt.Errorf("read execution result: %w", err)
	}

	return stored, nil
}

func (r *changeSetRepository) SaveChange(
	ctx context.Context,
	change entity.ExecutionChange,
) (entity.ExecutionChange, error) {
	saved, err := scanChange(r.db.Querier(ctx).QueryRowContext(
		ctx,
		saveChangeQuery,
		change.ExecutionID,
		change.WorkspaceID.String(),
		change.Repository,
		change.Branch,
		change.BaseSHA,
		change.HeadSHA,
		change.Commits,
		change.Additions,
		change.Deletions,
		change.FilesChanged,
		optionalID(change.DiffArtifactID),
		change.PullRequestURL,
		change.ReportedAt,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return r.change(ctx, change.ExecutionID, change.Repository)
	}

	if err != nil {
		return entity.ExecutionChange{}, fmt.Errorf("save execution change: %w", err)
	}

	return saved, nil
}

func (r *changeSetRepository) change(
	ctx context.Context,
	executionID, repositoryName string,
) (entity.ExecutionChange, error) {
	stored, err := scanChange(r.db.Querier(ctx).QueryRowContext(
		ctx, changeByRepositoryQuery, executionID, repositoryName,
	))
	if err != nil {
		return entity.ExecutionChange{}, fmt.Errorf("read execution change: %w", err)
	}

	return stored, nil
}

func (r *changeSetRepository) SaveValidation(
	ctx context.Context,
	validation entity.ExecutionValidation,
) (entity.ExecutionValidation, error) {
	saved, err := scanValidation(r.db.Querier(ctx).QueryRowContext(
		ctx,
		saveValidationQuery,
		validation.ExecutionID,
		validation.WorkspaceID.String(),
		validation.Check,
		string(validation.Status),
		validation.Detail,
		optionalID(validation.ArtifactID),
		validation.ReportedAt,
	))

	if errors.Is(err, sql.ErrNoRows) {
		return r.validation(ctx, validation.ExecutionID, validation.Check)
	}

	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("save execution validation: %w", err)
	}

	return saved, nil
}

func (r *changeSetRepository) validation(
	ctx context.Context,
	executionID, check string,
) (entity.ExecutionValidation, error) {
	stored, err := scanValidation(r.db.Querier(ctx).QueryRowContext(
		ctx, validationByCheckQuery, executionID, check,
	))
	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("read execution validation: %w", err)
	}

	return stored, nil
}

func (r *changeSetRepository) LinkChange(
	ctx context.Context,
	changeID, codeLinkID uuid.UUID,
) error {
	if _, err := r.db.Querier(ctx).ExecContext(
		ctx, linkChangeQuery, changeID.String(), codeLinkID.String(),
	); err != nil {
		return fmt.Errorf("attach a code link to an execution change: %w", err)
	}

	return nil
}

func (r *changeSetRepository) Get(
	ctx context.Context,
	executionID string,
) (entity.ExecutionChangeSet, error) {
	result, err := r.result(ctx, executionID)
	if err != nil && !errors.Is(err, entity.ErrExecutionResultNotFound) {
		return entity.ExecutionChangeSet{}, err
	}

	changes, err := r.changes(ctx, executionID)
	if err != nil {
		return entity.ExecutionChangeSet{}, err
	}

	validations, err := r.validations(ctx, executionID)
	if err != nil {
		return entity.ExecutionChangeSet{}, err
	}

	return entity.ExecutionChangeSet{
		Result:      result,
		Changes:     changes,
		Validations: validations,
	}, nil
}

func (r *changeSetRepository) changes(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionChange, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, changesQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("list execution changes: %w", err)
	}

	defer func() { _ = rows.Close() }()

	changes := make([]entity.ExecutionChange, 0)

	for rows.Next() {
		change, err := scanChange(rows)
		if err != nil {
			return nil, fmt.Errorf("read an execution change: %w", err)
		}

		changes = append(changes, change)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read execution changes: %w", err)
	}

	return changes, nil
}

func (r *changeSetRepository) validations(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionValidation, error) {
	rows, err := r.db.Querier(ctx).QueryContext(ctx, validationsQuery, executionID)
	if err != nil {
		return nil, fmt.Errorf("list execution validations: %w", err)
	}

	defer func() { _ = rows.Close() }()

	validations := make([]entity.ExecutionValidation, 0)

	for rows.Next() {
		validation, err := scanValidation(rows)
		if err != nil {
			return nil, fmt.Errorf("read an execution validation: %w", err)
		}

		validations = append(validations, validation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read execution validations: %w", err)
	}

	return validations, nil
}

func (r *changeSetRepository) ByIssue(
	ctx context.Context,
	workspaceID, issueID uuid.UUID,
) (entity.IssueChangeSet, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, issueChangesQuery, workspaceID.String(), issueID.String(),
	)
	if err != nil {
		return entity.IssueChangeSet{}, fmt.Errorf("list the changes on an issue: %w", err)
	}

	defer func() { _ = rows.Close() }()

	changes := make([]entity.IssueRepositoryChange, 0)

	for rows.Next() {
		change, err := scanIssueChange(rows)
		if err != nil {
			return entity.IssueChangeSet{}, fmt.Errorf("read a change on an issue: %w", err)
		}

		changes = append(changes, change)
	}

	if err := rows.Err(); err != nil {
		return entity.IssueChangeSet{}, fmt.Errorf("read the changes on an issue: %w", err)
	}

	return entity.IssueChangeSet{IssueID: issueID, Changes: changes}, nil
}

func scanResult(row scanner) (entity.ExecutionResult, error) {
	var (
		result      entity.ExecutionResult
		workspaceID string
	)

	if err := row.Scan(
		&result.ExecutionID,
		&workspaceID,
		&result.Summary,
		&result.ReportedAt,
		&result.CreatedAt,
		&result.UpdatedAt,
	); err != nil {
		return entity.ExecutionResult{}, err
	}

	parsed, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.ExecutionResult{}, fmt.Errorf("parse workspace id: %w", err)
	}

	result.WorkspaceID = parsed

	return result, nil
}

func scanChange(row scanner) (entity.ExecutionChange, error) {
	change, _, err := readChange(row, false)

	return change, err
}

func scanIssueChange(row scanner) (entity.IssueRepositoryChange, error) {
	change, attempt, err := readChange(row, true)
	if err != nil {
		return entity.IssueRepositoryChange{}, err
	}

	return entity.IssueRepositoryChange{ExecutionChange: change, Attempt: attempt}, nil
}

func readChange(row scanner, withAttempt bool) (entity.ExecutionChange, int, error) {
	var (
		change      entity.ExecutionChange
		changeID    string
		workspaceID string
		artifactID  string
		codeLinkID  string
		attempt     int
	)

	targets := []any{
		&changeID,
		&change.ExecutionID,
		&workspaceID,
		&change.Repository,
		&change.Branch,
		&change.BaseSHA,
		&change.HeadSHA,
		&change.Commits,
		&change.Additions,
		&change.Deletions,
		&change.FilesChanged,
		&artifactID,
		&change.PullRequestURL,
		&codeLinkID,
		&change.ReportedAt,
		&change.CreatedAt,
		&change.UpdatedAt,
	}

	if withAttempt {
		targets = append(targets, &attempt)
	}

	if err := row.Scan(targets...); err != nil {
		return entity.ExecutionChange{}, 0, err
	}

	parsedID, err := uuid.Parse(changeID)
	if err != nil {
		return entity.ExecutionChange{}, 0, fmt.Errorf("parse change id: %w", err)
	}

	parsedWorkspace, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.ExecutionChange{}, 0, fmt.Errorf("parse workspace id: %w", err)
	}

	parsedArtifact, err := parseOptionalID(artifactID)
	if err != nil {
		return entity.ExecutionChange{}, 0, fmt.Errorf("parse diff artifact id: %w", err)
	}

	parsedLink, err := parseOptionalID(codeLinkID)
	if err != nil {
		return entity.ExecutionChange{}, 0, fmt.Errorf("parse code link id: %w", err)
	}

	change.ID = parsedID
	change.WorkspaceID = parsedWorkspace
	change.DiffArtifactID = parsedArtifact
	change.CodeLinkID = parsedLink

	return change, attempt, nil
}

func scanValidation(row scanner) (entity.ExecutionValidation, error) {
	var (
		validation   entity.ExecutionValidation
		validationID string
		workspaceID  string
		status       string
		artifactID   string
	)

	if err := row.Scan(
		&validationID,
		&validation.ExecutionID,
		&workspaceID,
		&validation.Check,
		&status,
		&validation.Detail,
		&artifactID,
		&validation.ReportedAt,
		&validation.CreatedAt,
		&validation.UpdatedAt,
	); err != nil {
		return entity.ExecutionValidation{}, err
	}

	parsedID, err := uuid.Parse(validationID)
	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("parse validation id: %w", err)
	}

	parsedWorkspace, err := uuid.Parse(workspaceID)
	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("parse workspace id: %w", err)
	}

	parsedArtifact, err := parseOptionalID(artifactID)
	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("parse artifact id: %w", err)
	}

	validation.ID = parsedID
	validation.WorkspaceID = parsedWorkspace
	validation.Status = entity.ValidationStatus(status)
	validation.ArtifactID = parsedArtifact

	return validation, nil
}

func optionalID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func parseOptionalID(raw string) (uuid.UUID, error) {
	if raw == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(raw)
}
