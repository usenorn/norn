package issuetemplate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const uniqueViolationCode = "23505"

const templateColumns = `
       t.id,
       t.workspace_id,
       coalesce(t.team_id::text, ''),
       t.name,
       t.description,
       t.title,
       t.body,
       coalesce(t.body_doc::text, ''),
       array_to_string(t.required_fields, ','),
       coalesce(t.state_id::text, ''),
       coalesce(t.project_id::text, ''),
       coalesce(t.assignee_account_id::text, ''),
       array_to_string(t.label_ids, ','),
       t.priority,
       t.estimate,
       t.position,
       coalesce(t.created_by_account_id::text, ''),
       t.created_at,
       t.updated_at`

const createTemplateQuery = `
WITH created AS (
    INSERT INTO workspace_issue_templates
        (id, workspace_id, team_id, name, description, title, body, body_doc, required_fields,
         state_id, project_id, assignee_account_id, label_ids, priority, estimate, position,
         created_by_account_id, created_at, updated_at)
    VALUES
        ($1, $2, nullif($3, '')::uuid, $4, $5, $6, $7, $8::jsonb, $9::text[],
         nullif($10, '')::uuid, nullif($11, '')::uuid, nullif($12, '')::uuid, $13::uuid[], $14,
         $15, $16, nullif($17, '')::uuid, $18, $18)
    RETURNING *
)
SELECT` + templateColumns + `
FROM created t`

const updateTemplateQuery = `
WITH updated AS (
    UPDATE workspace_issue_templates
    SET name                = $3,
        description         = $4,
        title               = $5,
        body                = $6,
        body_doc            = $7::jsonb,
        required_fields     = $8::text[],
        state_id            = nullif($9, '')::uuid,
        project_id          = nullif($10, '')::uuid,
        assignee_account_id = nullif($11, '')::uuid,
        label_ids           = $12::uuid[],
        priority            = $13,
        estimate            = $14,
        position            = $15,
        updated_at          = $16
    WHERE id = $1 AND workspace_id = $2
    RETURNING *
)
SELECT` + templateColumns + `
FROM updated t`

const listTemplatesQuery = `
SELECT` + templateColumns + `
FROM workspace_issue_templates t
WHERE t.workspace_id = $1
  AND (t.team_id IS NULL OR t.team_id = nullif($2, '')::uuid)
ORDER BY t.team_id NULLS FIRST, t.position, lower(t.name)`

const templateByIDQuery = `
SELECT` + templateColumns + `
FROM workspace_issue_templates t
WHERE t.id = $1 AND t.workspace_id = $2`

const countTemplatesQuery = `
SELECT count(*)
FROM workspace_issue_templates
WHERE workspace_id = $1 AND team_id IS NOT DISTINCT FROM nullif($2, '')::uuid`

const removeTemplateQuery = `
DELETE FROM workspace_issue_templates
WHERE id = $1 AND workspace_id = $2`

type templateRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueTemplate {
	return &templateRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTemplate(row scanner) (entity.IssueTemplate, error) {
	var (
		template                  entity.IssueTemplate
		id, workspace, team       string
		document, required        string
		state, project, assignee  string
		labels, priority, creator string
		estimate                  sql.NullInt64
	)

	if err := row.Scan(
		&id, &workspace, &team, &template.Name, &template.Description, &template.Title,
		&template.Body, &document, &required, &state, &project, &assignee, &labels,
		&priority, &estimate, &template.Position, &creator,
		&template.CreatedAt, &template.UpdatedAt,
	); err != nil {
		return entity.IssueTemplate{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.IssueTemplate{}, fmt.Errorf("parse template id: %w", err)
	}

	template.ID = parsed

	if template.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.IssueTemplate{}, fmt.Errorf("parse template workspace id: %w", err)
	}

	for target, raw := range map[*uuid.UUID]string{
		&template.TeamID:             team,
		&template.StateID:            state,
		&template.ProjectID:          project,
		&template.AssigneeAccountID:  assignee,
		&template.CreatedByAccountID: creator,
	} {
		if raw == "" {
			continue
		}

		if *target, err = uuid.Parse(raw); err != nil {
			return entity.IssueTemplate{}, fmt.Errorf("parse template reference: %w", err)
		}
	}

	if template.BodyDoc, err = entity.DecodeDocument([]byte(document)); err != nil {
		return entity.IssueTemplate{}, fmt.Errorf("decode template body: %w", err)
	}

	if template.LabelIDs, err = identifiers(labels); err != nil {
		return entity.IssueTemplate{}, fmt.Errorf("parse template labels: %w", err)
	}

	template.RequiredFields = fields(required)
	template.Priority = entity.IssuePriority(priority)

	if estimate.Valid {
		template.Estimate = int(estimate.Int64)
	}

	return template, nil
}

func fields(joined string) []entity.TemplateField {
	if joined == "" {
		return nil
	}

	raw := strings.Split(joined, ",")

	named := make([]entity.TemplateField, 0, len(raw))
	for _, value := range raw {
		named = append(named, entity.TemplateField(value))
	}

	return named
}

func identifiers(joined string) ([]uuid.UUID, error) {
	if joined == "" {
		return nil, nil
	}

	raw := strings.Split(joined, ",")

	parsed := make([]uuid.UUID, 0, len(raw))

	for _, value := range raw {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, err
		}

		parsed = append(parsed, id)
	}

	return parsed, nil
}

func (r *templateRepository) written(template entity.IssueTemplate) ([]any, error) {
	document := any(nil)
	if template.BodyDoc.Type != "" {
		encoded, err := template.BodyDoc.Encode()
		if err != nil {
			return nil, fmt.Errorf("encode template body: %w", err)
		}

		document = string(encoded)
	}

	estimate := any(nil)
	if template.Estimate > 0 {
		estimate = template.Estimate
	}

	required := make([]string, 0, len(template.RequiredFields))
	for _, field := range template.RequiredFields {
		required = append(required, string(field))
	}

	labels := make([]string, 0, len(template.LabelIDs))
	for _, id := range template.LabelIDs {
		labels = append(labels, id.String())
	}

	if template.Priority == "" {
		template.Priority = entity.IssuePriorityNone
	}

	return []any{
		template.Name, template.Description, template.Title, template.Body, document,
		required, text(template.StateID), text(template.ProjectID),
		text(template.AssigneeAccountID), labels, string(template.Priority), estimate,
		template.Position,
	}, nil
}

func (r *templateRepository) Create(
	ctx context.Context,
	template entity.IssueTemplate,
) (entity.IssueTemplate, error) {
	if template.ID == uuid.Nil {
		template.ID = uuid.New()
	}

	shared, err := r.written(template)
	if err != nil {
		return entity.IssueTemplate{}, err
	}

	args := append(
		[]any{template.ID.String(), template.WorkspaceID.String(), text(template.TeamID)},
		shared...,
	)
	args = append(args, text(template.CreatedByAccountID), template.UpdatedAt)

	created, err := scanTemplate(r.db.Querier(ctx).QueryRowContext(ctx, createTemplateQuery, args...))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return entity.IssueTemplate{}, entity.ErrIssueTemplateNameUsed
		}

		return entity.IssueTemplate{}, fmt.Errorf("create template: %w", err)
	}

	return created, nil
}

func (r *templateRepository) Update(
	ctx context.Context,
	template entity.IssueTemplate,
) (entity.IssueTemplate, error) {
	shared, err := r.written(template)
	if err != nil {
		return entity.IssueTemplate{}, err
	}

	args := append([]any{template.ID.String(), template.WorkspaceID.String()}, shared...)
	args = append(args, template.UpdatedAt)

	updated, err := scanTemplate(r.db.Querier(ctx).QueryRowContext(ctx, updateTemplateQuery, args...))
	if err != nil {
		var pgErr *pgconn.PgError

		switch {
		case errors.Is(err, sql.ErrNoRows):
			return entity.IssueTemplate{}, entity.ErrIssueTemplateNotFound
		case errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode:
			return entity.IssueTemplate{}, entity.ErrIssueTemplateNameUsed
		default:
			return entity.IssueTemplate{}, fmt.Errorf("update template: %w", err)
		}
	}

	return updated, nil
}

func (r *templateRepository) ListForTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) ([]entity.IssueTemplate, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listTemplatesQuery, workspaceID.String(), text(teamID),
	)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var templates []entity.IssueTemplate

	for rows.Next() {
		template, err := scanTemplate(rows)
		if err != nil {
			return nil, fmt.Errorf("read template: %w", err)
		}

		templates = append(templates, template)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read templates: %w", err)
	}

	return templates, nil
}

func (r *templateRepository) GetByID(
	ctx context.Context,
	workspaceID, templateID uuid.UUID,
) (entity.IssueTemplate, error) {
	template, err := scanTemplate(r.db.Querier(ctx).QueryRowContext(
		ctx, templateByIDQuery, templateID.String(), workspaceID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueTemplate{}, entity.ErrIssueTemplateNotFound
		}

		return entity.IssueTemplate{}, fmt.Errorf("find template: %w", err)
	}

	return template, nil
}

func (r *templateRepository) CountForTeam(
	ctx context.Context,
	workspaceID, teamID uuid.UUID,
) (int, error) {
	var counted int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, countTemplatesQuery, workspaceID.String(), text(teamID),
	).Scan(&counted); err != nil {
		return 0, fmt.Errorf("count templates: %w", err)
	}

	return counted, nil
}

func (r *templateRepository) Remove(
	ctx context.Context,
	workspaceID, templateID uuid.UUID,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, removeTemplateQuery, templateID.String(), workspaceID.String(),
	)
	if err != nil {
		return fmt.Errorf("remove template: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove template: %w", err)
	}

	if affected == 0 {
		return entity.ErrIssueTemplateNotFound
	}

	return nil
}

func text(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}
