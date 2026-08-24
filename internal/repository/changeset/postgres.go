package changeset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aarondl/null/v8"
	"github.com/aarondl/sqlboiler/v4/queries/qm"
	"github.com/google/uuid"

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
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

type changeSetRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.ChangeSet {
	return &changeSetRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func resultOf(model *dbpostgres.WorkspaceExecutionResult) (entity.ExecutionResult, error) {
	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.ExecutionResult{}, fmt.Errorf("parse workspace id: %w", err)
	}

	return entity.ExecutionResult{
		ExecutionID: model.ExecutionID,
		WorkspaceID: workspaceID,
		Summary:     model.Summary,
		ReportedAt:  model.ReportedAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func changeOf(model *dbpostgres.WorkspaceExecutionChange) (entity.ExecutionChange, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.ExecutionChange{}, fmt.Errorf("parse execution change id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.ExecutionChange{}, fmt.Errorf("parse workspace id: %w", err)
	}

	diffArtifactID, err := parseNullID(model.DiffArtifactID)
	if err != nil {
		return entity.ExecutionChange{}, fmt.Errorf("parse diff artifact id: %w", err)
	}

	codeLinkID, err := parseNullID(model.CodeLinkID)
	if err != nil {
		return entity.ExecutionChange{}, fmt.Errorf("parse code link id: %w", err)
	}

	return entity.ExecutionChange{
		ID:             id,
		ExecutionID:    model.ExecutionID,
		WorkspaceID:    workspaceID,
		Repository:     model.Repository,
		Branch:         model.Branch,
		BaseSHA:        model.BaseSha,
		HeadSHA:        model.HeadSha,
		Commits:        model.Commits,
		Additions:      model.Additions,
		Deletions:      model.Deletions,
		FilesChanged:   model.FilesChanged,
		DiffArtifactID: diffArtifactID,
		PullRequestURL: model.PullRequestURL,
		CodeLinkID:     codeLinkID,
		ReportedAt:     model.ReportedAt,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}, nil
}

func changesOf(
	models dbpostgres.WorkspaceExecutionChangeSlice,
) ([]entity.ExecutionChange, error) {
	changes := make([]entity.ExecutionChange, 0, len(models))

	for _, model := range models {
		change, err := changeOf(model)
		if err != nil {
			return nil, err
		}

		changes = append(changes, change)
	}

	return changes, nil
}

func validationOf(
	model *dbpostgres.WorkspaceExecutionValidation,
) (entity.ExecutionValidation, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("parse execution validation id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("parse workspace id: %w", err)
	}

	artifactID, err := parseNullID(model.ArtifactID)
	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("parse artifact id: %w", err)
	}

	return entity.ExecutionValidation{
		ID:          id,
		ExecutionID: model.ExecutionID,
		WorkspaceID: workspaceID,
		Check:       model.CheckName,
		Status:      entity.ValidationStatus(model.Status),
		Detail:      model.Detail,
		ArtifactID:  artifactID,
		ReportedAt:  model.ReportedAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func validationsOf(
	models dbpostgres.WorkspaceExecutionValidationSlice,
) ([]entity.ExecutionValidation, error) {
	validations := make([]entity.ExecutionValidation, 0, len(models))

	for _, model := range models {
		validation, err := validationOf(model)
		if err != nil {
			return nil, err
		}

		validations = append(validations, validation)
	}

	return validations, nil
}

func parseNullID(value null.String) (uuid.UUID, error) {
	if !value.Valid || value.String == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(value.String)
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
	model, err := dbpostgres.FindWorkspaceExecutionResult(ctx, r.db.Querier(ctx), executionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ExecutionResult{}, entity.ErrExecutionResultNotFound
		}

		return entity.ExecutionResult{}, fmt.Errorf("read execution result: %w", err)
	}

	return resultOf(model)
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
	model, err := dbpostgres.WorkspaceExecutionChanges(
		dbpostgres.WorkspaceExecutionChangeWhere.ExecutionID.EQ(executionID),
		dbpostgres.WorkspaceExecutionChangeWhere.Repository.EQ(repositoryName),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		return entity.ExecutionChange{}, fmt.Errorf("read execution change: %w", err)
	}

	return changeOf(model)
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
	model, err := dbpostgres.WorkspaceExecutionValidations(
		dbpostgres.WorkspaceExecutionValidationWhere.ExecutionID.EQ(executionID),
		dbpostgres.WorkspaceExecutionValidationWhere.CheckName.EQ(check),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		return entity.ExecutionValidation{}, fmt.Errorf("read execution validation: %w", err)
	}

	return validationOf(model)
}

func (r *changeSetRepository) LinkChange(
	ctx context.Context,
	changeID, codeLinkID uuid.UUID,
) error {
	if _, err := dbpostgres.WorkspaceExecutionChanges(
		dbpostgres.WorkspaceExecutionChangeWhere.ID.EQ(changeID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceExecutionChangeColumns.CodeLinkID: codeLinkID.String(),
		dbpostgres.WorkspaceExecutionChangeColumns.UpdatedAt:  time.Now().UTC(),
	}); err != nil {
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
		ExecutionID: executionID,
		Result:      result,
		Changes:     changes,
		Validations: validations,
	}, nil
}

func (r *changeSetRepository) changes(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionChange, error) {
	models, err := dbpostgres.WorkspaceExecutionChanges(
		dbpostgres.WorkspaceExecutionChangeWhere.ExecutionID.EQ(executionID),
		qm.OrderBy(
			dbpostgres.WorkspaceExecutionChangeColumns.Repository+", "+
				dbpostgres.WorkspaceExecutionChangeColumns.ID,
		),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list execution changes: %w", err)
	}

	return changesOf(models)
}

func (r *changeSetRepository) validations(
	ctx context.Context,
	executionID string,
) ([]entity.ExecutionValidation, error) {
	models, err := dbpostgres.WorkspaceExecutionValidations(
		dbpostgres.WorkspaceExecutionValidationWhere.ExecutionID.EQ(executionID),
		qm.OrderBy(
			dbpostgres.WorkspaceExecutionValidationColumns.CheckName+", "+
				dbpostgres.WorkspaceExecutionValidationColumns.ID,
		),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list execution validations: %w", err)
	}

	return validationsOf(models)
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
