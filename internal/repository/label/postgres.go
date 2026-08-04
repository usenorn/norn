package label

import (
	"context"
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
	uniqueViolationCode     = "23505"
	foreignKeyViolationCode = "23503"

	workspaceNameUniqueIndex = "workspace_labels_workspace_name_key"
	teamNameUniqueIndex      = "workspace_labels_team_name_key"
	groupExclusiveIndex      = "workspace_issue_labels_issue_group_key"
	issueTeamForeignKey      = "workspace_issue_labels_issue_team_fkey"
	labelForeignKey          = "workspace_issue_labels_label_fkey"
	groupForeignKey          = "workspace_labels_group_fkey"
)

const scopedListQuery = `
SELECT id, workspace_id, coalesce(team_id::text, ''), coalesce(group_id::text, ''), name, color, created_at, updated_at
FROM workspace_labels
WHERE workspace_id = $1
  AND (team_id IS NULL OR $2::boolean IS TRUE OR team_id = ANY($3::uuid[]))
ORDER BY lower(name), id`

const usageQuery = `
SELECT count(*)
FROM workspace_issue_labels il
JOIN workspace_issues i ON i.id = il.issue_id
WHERE il.label_id = $1
  AND ($2::boolean IS TRUE OR i.team_id = ANY($3::uuid[]))`

const moveApplicationsQuery = `
INSERT INTO workspace_issue_labels (
    workspace_id, issue_id, label_id, label_team_id, label_group_id, created_at
)
SELECT il.workspace_id, il.issue_id, $2, $3, $4, il.created_at
FROM workspace_issue_labels il
WHERE il.label_id = $1
ON CONFLICT (issue_id, label_id) DO NOTHING`

const dropApplicationsQuery = `DELETE FROM workspace_issue_labels WHERE label_id = $1`

const clearOtherApplicationsQuery = `
DELETE FROM workspace_issue_labels
WHERE issue_id = $1 AND label_id <> ALL($2::uuid[])`

const syncApplicationGroupQuery = `
UPDATE workspace_issue_labels
SET label_group_id = $2
WHERE label_id = $1 AND label_group_id IS DISTINCT FROM $2`

func optionalID(id uuid.UUID) null.String {
	if id == uuid.Nil {
		return null.NewString("", false)
	}

	return null.StringFrom(id.String())
}

func parseOptionalID(value null.String) (uuid.UUID, error) {
	if !value.Valid || value.String == "" {
		return uuid.Nil, nil
	}

	return uuid.Parse(value.String)
}

func toEntity(model *dbpostgres.WorkspaceLabel) (entity.Label, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.Label{}, fmt.Errorf("parse label id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.Label{}, fmt.Errorf("parse label workspace id: %w", err)
	}

	teamID, err := parseOptionalID(model.TeamID)
	if err != nil {
		return entity.Label{}, fmt.Errorf("parse label team id: %w", err)
	}

	groupID, err := parseOptionalID(model.GroupID)
	if err != nil {
		return entity.Label{}, fmt.Errorf("parse label group id: %w", err)
	}

	return entity.Label{
		ID:          id,
		WorkspaceID: workspaceID,
		TeamID:      teamID,
		GroupID:     groupID,
		Name:        model.Name,
		Color:       entity.LabelColor(model.Color),
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func toModel(label entity.Label) *dbpostgres.WorkspaceLabel {
	return &dbpostgres.WorkspaceLabel{
		ID:          label.ID.String(),
		WorkspaceID: label.WorkspaceID.String(),
		TeamID:      optionalID(label.TeamID),
		GroupID:     optionalID(label.GroupID),
		Name:        label.Name,
		Color:       string(label.Color),
		CreatedAt:   label.CreatedAt,
		UpdatedAt:   label.UpdatedAt,
	}
}

func toEntities(models dbpostgres.WorkspaceLabelSlice) ([]entity.Label, error) {
	labels := make([]entity.Label, 0, len(models))

	for _, model := range models {
		label, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		labels = append(labels, label)
	}

	return labels, nil
}

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch {
	case pgErr.Code == uniqueViolationCode &&
		(pgErr.ConstraintName == workspaceNameUniqueIndex || pgErr.ConstraintName == teamNameUniqueIndex):
		return entity.ErrLabelNameTaken
	case pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == groupExclusiveIndex:
		return entity.ErrLabelGroupExclusive
	case pgErr.Code == foreignKeyViolationCode && pgErr.ConstraintName == issueTeamForeignKey:
		return entity.ErrLabelOutOfScope
	case pgErr.Code == foreignKeyViolationCode && pgErr.ConstraintName == labelForeignKey:
		return entity.ErrLabelNotFound
	case pgErr.Code == foreignKeyViolationCode && pgErr.ConstraintName == groupForeignKey:
		return entity.ErrLabelGroupNotFound
	default:
		return err
	}
}

type labelRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Label {
	return &labelRepository{db: db}
}

func (r *labelRepository) Create(ctx context.Context, label entity.Label) (entity.Label, error) {
	if label.ID == uuid.Nil {
		label.ID = uuid.New()
	}

	now := time.Now().UTC()
	label.CreatedAt = now
	label.UpdatedAt = now

	model := toModel(label)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.Label{}, translated
		}

		return entity.Label{}, fmt.Errorf("insert label: %w", err)
	}

	return toEntity(model)
}

func (r *labelRepository) find(ctx context.Context, workspaceID, id uuid.UUID, mods ...qm.QueryMod) (entity.Label, error) {
	query := append([]qm.QueryMod{
		dbpostgres.WorkspaceLabelWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceLabelWhere.WorkspaceID.EQ(workspaceID.String()),
	}, mods...)

	model, err := dbpostgres.WorkspaceLabels(query...).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Label{}, entity.ErrLabelNotFound
		}

		return entity.Label{}, fmt.Errorf("find label: %w", err)
	}

	return toEntity(model)
}

func (r *labelRepository) GetByID(ctx context.Context, workspaceID, id uuid.UUID) (entity.Label, error) {
	return r.find(ctx, workspaceID, id)
}

func (r *labelRepository) LockByID(ctx context.Context, workspaceID, id uuid.UUID) (entity.Label, error) {
	return r.find(ctx, workspaceID, id, qm.For("UPDATE"))
}

func (r *labelRepository) ListByWorkspaceID(
	ctx context.Context,
	workspaceID uuid.UUID,
	scope entity.TeamScope,
) ([]entity.Label, error) {
	teamIDs := make([]string, 0, len(scope.TeamIDs))
	for _, teamID := range scope.TeamIDs {
		teamIDs = append(teamIDs, teamID.String())
	}

	rows, err := r.db.Querier(ctx).QueryContext(
		ctx,
		scopedListQuery,
		workspaceID.String(),
		scope.AllTeams,
		teamIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}

	defer func() { _ = rows.Close() }()

	labels := make([]entity.Label, 0)

	for rows.Next() {
		label, err := scanLabel(rows)
		if err != nil {
			return nil, err
		}

		labels = append(labels, label)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate labels: %w", err)
	}

	return labels, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanLabel(row scanner) (entity.Label, error) {
	var (
		label     entity.Label
		id        string
		workspace string
		team      string
		group     string
		color     string
	)

	if err := row.Scan(
		&id,
		&workspace,
		&team,
		&group,
		&label.Name,
		&color,
		&label.CreatedAt,
		&label.UpdatedAt,
	); err != nil {
		return entity.Label{}, fmt.Errorf("scan label: %w", err)
	}

	var err error

	if label.ID, err = uuid.Parse(id); err != nil {
		return entity.Label{}, fmt.Errorf("parse label id: %w", err)
	}

	if label.WorkspaceID, err = uuid.Parse(workspace); err != nil {
		return entity.Label{}, fmt.Errorf("parse label workspace id: %w", err)
	}

	if team != "" {
		if label.TeamID, err = uuid.Parse(team); err != nil {
			return entity.Label{}, fmt.Errorf("parse label team id: %w", err)
		}
	}

	if group != "" {
		if label.GroupID, err = uuid.Parse(group); err != nil {
			return entity.Label{}, fmt.Errorf("parse label group id: %w", err)
		}
	}

	label.Color = entity.LabelColor(color)

	return label, nil
}

func (r *labelRepository) ListByIDs(
	ctx context.Context,
	workspaceID uuid.UUID,
	ids []uuid.UUID,
) ([]entity.Label, error) {
	if len(ids) == 0 {
		return []entity.Label{}, nil
	}

	values := make([]any, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.String())
	}

	models, err := dbpostgres.WorkspaceLabels(
		dbpostgres.WorkspaceLabelWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.WhereIn(dbpostgres.WorkspaceLabelColumns.ID+" IN ?", values...),
		qm.OrderBy("lower("+dbpostgres.WorkspaceLabelColumns.Name+"), "+dbpostgres.WorkspaceLabelColumns.ID),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list labels by id: %w", err)
	}

	return toEntities(models)
}

func (r *labelRepository) UpdateSettings(
	ctx context.Context,
	id uuid.UUID,
	name string,
	color entity.LabelColor,
	groupID uuid.UUID,
) (entity.Label, error) {
	updated, err := dbpostgres.WorkspaceLabels(
		dbpostgres.WorkspaceLabelWhere.ID.EQ(id.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceLabelColumns.Name:      name,
		dbpostgres.WorkspaceLabelColumns.Color:     string(color),
		dbpostgres.WorkspaceLabelColumns.GroupID:   optionalID(groupID),
		dbpostgres.WorkspaceLabelColumns.UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.Label{}, translated
		}

		return entity.Label{}, fmt.Errorf("update label: %w", err)
	}

	if updated == 0 {
		return entity.Label{}, entity.ErrLabelNotFound
	}

	model, err := dbpostgres.FindWorkspaceLabel(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		return entity.Label{}, fmt.Errorf("read updated label: %w", err)
	}

	return toEntity(model)
}

func (r *labelRepository) SyncApplicationGroup(ctx context.Context, labelID, groupID uuid.UUID) error {
	var group any
	if groupID != uuid.Nil {
		group = groupID.String()
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		syncApplicationGroupQuery,
		labelID.String(),
		group,
	); err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return translated
		}

		return fmt.Errorf("sync label applications: %w", err)
	}

	return nil
}

func (r *labelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	deleted, err := dbpostgres.WorkspaceLabels(
		dbpostgres.WorkspaceLabelWhere.ID.EQ(id.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete label: %w", err)
	}

	if deleted == 0 {
		return entity.ErrLabelNotFound
	}

	return nil
}

func (r *labelRepository) SetForIssue(ctx context.Context, issue entity.Issue, labels []entity.Label) error {
	keep := make([]string, 0, len(labels))
	for _, label := range labels {
		keep = append(keep, label.ID.String())
	}

	if _, err := r.db.Querier(ctx).ExecContext(
		ctx,
		clearOtherApplicationsQuery,
		issue.ID.String(),
		keep,
	); err != nil {
		return fmt.Errorf("clear issue labels: %w", err)
	}

	for _, label := range labels {
		application := &dbpostgres.WorkspaceIssueLabel{
			WorkspaceID:  issue.WorkspaceID.String(),
			IssueID:      issue.ID.String(),
			LabelID:      label.ID.String(),
			LabelTeamID:  optionalID(label.TeamID),
			LabelGroupID: optionalID(label.GroupID),
			CreatedAt:    time.Now().UTC(),
		}

		if err := application.Upsert(
			ctx,
			r.db.Querier(ctx),
			false,
			[]string{
				dbpostgres.WorkspaceIssueLabelColumns.IssueID,
				dbpostgres.WorkspaceIssueLabelColumns.LabelID,
			},
			boil.None(),
			boil.Infer(),
		); err != nil {
			if translated := translateWriteError(err); !errors.Is(translated, err) {
				return translated
			}

			return fmt.Errorf("apply issue label: %w", err)
		}
	}

	return nil
}

func (r *labelRepository) Usage(
	ctx context.Context,
	labelID uuid.UUID,
	scope entity.TeamScope,
) (entity.LabelUsage, error) {
	teamIDs := make([]string, 0, len(scope.TeamIDs))
	for _, teamID := range scope.TeamIDs {
		teamIDs = append(teamIDs, teamID.String())
	}

	var usage entity.LabelUsage

	if err := r.db.Querier(ctx).QueryRowContext(
		ctx,
		usageQuery,
		labelID.String(),
		scope.AllTeams,
		teamIDs,
	).Scan(&usage.Issues); err != nil {
		return entity.LabelUsage{}, fmt.Errorf("count label usage: %w", err)
	}

	return usage, nil
}

func (r *labelRepository) MoveApplications(ctx context.Context, source, target entity.Label) error {
	querier := r.db.Querier(ctx)

	if _, err := querier.ExecContext(
		ctx,
		moveApplicationsQuery,
		source.ID.String(),
		target.ID.String(),
		optionalID(target.TeamID),
		optionalID(target.GroupID),
	); err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return translated
		}

		return fmt.Errorf("move label applications: %w", err)
	}

	if _, err := querier.ExecContext(ctx, dropApplicationsQuery, source.ID.String()); err != nil {
		return fmt.Errorf("drop label applications: %w", err)
	}

	return nil
}
