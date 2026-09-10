package issuedraft

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

const draftColumns = `
       d.id,
       d.workspace_id,
       d.account_id,
       coalesce(d.team_id::text, ''),
       d.title,
       d.description,
       coalesce(d.description_doc::text, ''),
       coalesce(d.state_id::text, ''),
       coalesce(d.project_id::text, ''),
       coalesce(d.cycle_id::text, ''),
       coalesce(d.assignee_account_id::text, ''),
       coalesce(d.parent_issue_id::text, ''),
       array_to_string(d.label_ids, ','),
       array_to_string(d.attachment_ids, ','),
       d.priority,
       d.estimate,
       coalesce(to_char(d.due_on, 'YYYY-MM-DD'), ''),
       d.created_at,
       d.updated_at`

const saveDraftQuery = `
WITH saved AS (
    INSERT INTO workspace_issue_drafts
        (id, workspace_id, account_id, team_id, title, description, description_doc, state_id,
         project_id, cycle_id, assignee_account_id, parent_issue_id, label_ids, attachment_ids,
         priority, estimate, due_on, created_at, updated_at)
    VALUES
        ($1, $2, $3, nullif($4, '')::uuid, $5, $6, $7::jsonb, nullif($8, '')::uuid,
         nullif($9, '')::uuid, nullif($10, '')::uuid, nullif($11, '')::uuid, nullif($12, '')::uuid,
         $13::uuid[], $14::uuid[], $15, $16, nullif($17, '')::date, $18, $18)
    ON CONFLICT (id) DO UPDATE SET
        team_id             = excluded.team_id,
        title               = excluded.title,
        description         = excluded.description,
        description_doc     = excluded.description_doc,
        state_id            = excluded.state_id,
        project_id          = excluded.project_id,
        cycle_id            = excluded.cycle_id,
        assignee_account_id = excluded.assignee_account_id,
        parent_issue_id     = excluded.parent_issue_id,
        label_ids           = excluded.label_ids,
        attachment_ids      = excluded.attachment_ids,
        priority            = excluded.priority,
        estimate            = excluded.estimate,
        due_on              = excluded.due_on,
        updated_at          = excluded.updated_at
    WHERE workspace_issue_drafts.account_id = excluded.account_id
      AND workspace_issue_drafts.workspace_id = excluded.workspace_id
    RETURNING *
)
SELECT` + draftColumns + `
FROM saved d`

const listDraftsQuery = `
SELECT` + draftColumns + `
FROM workspace_issue_drafts d
WHERE d.workspace_id = $1 AND d.account_id = $2
ORDER BY d.updated_at DESC, d.id DESC
LIMIT $3`

const draftByIDQuery = `
SELECT` + draftColumns + `
FROM workspace_issue_drafts d
WHERE d.id = $1 AND d.workspace_id = $2 AND d.account_id = $3`

const countDraftsQuery = `
SELECT count(*)
FROM workspace_issue_drafts
WHERE workspace_id = $1 AND account_id = $2`

const removeDraftQuery = `
DELETE FROM workspace_issue_drafts
WHERE id = $1 AND workspace_id = $2 AND account_id = $3`

type draftRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.IssueDraft {
	return &draftRepository{db: db}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDraft(row scanner) (entity.IssueDraft, error) {
	var (
		draft                        entity.IssueDraft
		id, workspace, account, team string
		document, state, project     string
		cycle, assignee, parent      string
		labels, attachments          string
		priority, dueOn              string
		estimate                     sql.NullInt64
	)

	if err := row.Scan(
		&id, &workspace, &account, &team, &draft.Title, &draft.Description, &document,
		&state, &project, &cycle, &assignee, &parent, &labels, &attachments,
		&priority, &estimate, &dueOn, &draft.CreatedAt, &draft.UpdatedAt,
	); err != nil {
		return entity.IssueDraft{}, err
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return entity.IssueDraft{}, fmt.Errorf("parse draft id: %w", err)
	}

	draft.ID = parsed

	if draft.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.IssueDraft{}, fmt.Errorf("parse draft workspace id: %w", err)
	}

	if draft.AccountID, err = uuid.Parse(account); err != nil {
		return entity.IssueDraft{}, fmt.Errorf("parse draft account id: %w", err)
	}

	for target, raw := range map[*uuid.UUID]string{
		&draft.TeamID:            team,
		&draft.StateID:           state,
		&draft.ProjectID:         project,
		&draft.CycleID:           cycle,
		&draft.AssigneeAccountID: assignee,
		&draft.ParentIssueID:     parent,
	} {
		if raw == "" {
			continue
		}

		if *target, err = uuid.Parse(raw); err != nil {
			return entity.IssueDraft{}, fmt.Errorf("parse draft reference: %w", err)
		}
	}

	if draft.DescriptionDoc, err = entity.DecodeDocument([]byte(document)); err != nil {
		return entity.IssueDraft{}, fmt.Errorf("decode draft description: %w", err)
	}

	if draft.LabelIDs, err = identifiers(labels); err != nil {
		return entity.IssueDraft{}, fmt.Errorf("parse draft labels: %w", err)
	}

	if draft.AttachmentIDs, err = identifiers(attachments); err != nil {
		return entity.IssueDraft{}, fmt.Errorf("parse draft attachments: %w", err)
	}

	draft.Priority = entity.IssuePriority(priority)
	draft.DueOn = dueOn

	if estimate.Valid {
		draft.Estimate = int(estimate.Int64)
	}

	return draft, nil
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

func (r *draftRepository) Save(
	ctx context.Context,
	draft entity.IssueDraft,
) (entity.IssueDraft, error) {
	if draft.ID == uuid.Nil {
		draft.ID = uuid.New()
	}

	if draft.Priority == "" {
		draft.Priority = entity.IssuePriorityNone
	}

	document := any(nil)
	if draft.DescriptionDoc.Type != "" {
		encoded, err := draft.DescriptionDoc.Encode()
		if err != nil {
			return entity.IssueDraft{}, fmt.Errorf("encode draft description: %w", err)
		}

		document = string(encoded)
	}

	estimate := any(nil)
	if draft.Estimate > 0 {
		estimate = draft.Estimate
	}

	saved, err := scanDraft(r.db.Querier(ctx).QueryRowContext(
		ctx, saveDraftQuery,
		draft.ID.String(), draft.WorkspaceID.String(), draft.AccountID.String(),
		text(draft.TeamID), draft.Title, draft.Description, document,
		text(draft.StateID), text(draft.ProjectID), text(draft.CycleID),
		text(draft.AssigneeAccountID), text(draft.ParentIssueID),
		written(draft.LabelIDs), written(draft.AttachmentIDs),
		string(draft.Priority), estimate, draft.DueOn, draft.UpdatedAt,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueDraft{}, entity.ErrIssueDraftNotFound
		}

		return entity.IssueDraft{}, fmt.Errorf("save draft: %w", err)
	}

	return saved, nil
}

func (r *draftRepository) List(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
	limit int,
) ([]entity.IssueDraft, error) {
	rows, err := r.db.Querier(ctx).QueryContext(
		ctx, listDraftsQuery, workspaceID.String(), accountID.String(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}

	defer func() { _ = rows.Close() }()

	drafts := make([]entity.IssueDraft, 0, limit)

	for rows.Next() {
		draft, err := scanDraft(rows)
		if err != nil {
			return nil, fmt.Errorf("read draft: %w", err)
		}

		drafts = append(drafts, draft)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read drafts: %w", err)
	}

	return drafts, nil
}

func (r *draftRepository) GetByID(
	ctx context.Context,
	workspaceID, accountID, draftID uuid.UUID,
) (entity.IssueDraft, error) {
	draft, err := scanDraft(r.db.Querier(ctx).QueryRowContext(
		ctx, draftByIDQuery, draftID.String(), workspaceID.String(), accountID.String(),
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.IssueDraft{}, entity.ErrIssueDraftNotFound
		}

		return entity.IssueDraft{}, fmt.Errorf("find draft: %w", err)
	}

	return draft, nil
}

func (r *draftRepository) Count(
	ctx context.Context,
	workspaceID, accountID uuid.UUID,
) (int, error) {
	var counted int

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx, countDraftsQuery, workspaceID.String(), accountID.String(),
	).Scan(&counted); err != nil {
		return 0, fmt.Errorf("count drafts: %w", err)
	}

	return counted, nil
}

func (r *draftRepository) Remove(
	ctx context.Context,
	workspaceID, accountID, draftID uuid.UUID,
) error {
	result, err := r.db.Querier(ctx).ExecContext(
		ctx, removeDraftQuery, draftID.String(), workspaceID.String(), accountID.String(),
	)
	if err != nil {
		return fmt.Errorf("remove draft: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove draft: %w", err)
	}

	if affected == 0 {
		return entity.ErrIssueDraftNotFound
	}

	return nil
}

func text(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	return id.String()
}

func written(ids []uuid.UUID) []string {
	raw := make([]string, 0, len(ids))

	for _, id := range ids {
		raw = append(raw, id.String())
	}

	return raw
}
