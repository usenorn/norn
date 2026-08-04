package labelgroup

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
	uniqueViolationCode     = "23505"
	foreignKeyViolationCode = "23503"

	nameUniqueIndex = "workspace_label_groups_workspace_name_key"
	labelForeignKey = "workspace_labels_group_fkey"
)

const ungroupQuery = `
UPDATE workspace_labels SET group_id = NULL, updated_at = $2 WHERE group_id = $1`

const ungroupApplicationsQuery = `
UPDATE workspace_issue_labels SET label_group_id = NULL WHERE label_group_id = $1`

func toEntity(model *dbpostgres.WorkspaceLabelGroup) (entity.LabelGroup, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.LabelGroup{}, fmt.Errorf("parse label group id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.LabelGroup{}, fmt.Errorf("parse label group workspace id: %w", err)
	}

	return entity.LabelGroup{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        model.Name,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch {
	case pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == nameUniqueIndex:
		return entity.ErrLabelGroupNameTaken
	case pgErr.Code == foreignKeyViolationCode && pgErr.ConstraintName == labelForeignKey:
		return entity.ErrLabelGroupInUse
	default:
		return err
	}
}

type labelGroupRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.LabelGroup {
	return &labelGroupRepository{db: db}
}

func (r *labelGroupRepository) Create(ctx context.Context, group entity.LabelGroup) (entity.LabelGroup, error) {
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}

	now := time.Now().UTC()

	model := &dbpostgres.WorkspaceLabelGroup{
		ID:          group.ID.String(),
		WorkspaceID: group.WorkspaceID.String(),
		Name:        group.Name,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.LabelGroup{}, translated
		}

		return entity.LabelGroup{}, fmt.Errorf("insert label group: %w", err)
	}

	return toEntity(model)
}

func (r *labelGroupRepository) GetByID(ctx context.Context, workspaceID, id uuid.UUID) (entity.LabelGroup, error) {
	model, err := dbpostgres.WorkspaceLabelGroups(
		dbpostgres.WorkspaceLabelGroupWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceLabelGroupWhere.WorkspaceID.EQ(workspaceID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.LabelGroup{}, entity.ErrLabelGroupNotFound
		}

		return entity.LabelGroup{}, fmt.Errorf("find label group: %w", err)
	}

	return toEntity(model)
}

func (r *labelGroupRepository) ListByWorkspaceID(
	ctx context.Context,
	workspaceID uuid.UUID,
) ([]entity.LabelGroup, error) {
	models, err := dbpostgres.WorkspaceLabelGroups(
		dbpostgres.WorkspaceLabelGroupWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.OrderBy("lower("+dbpostgres.WorkspaceLabelGroupColumns.Name+"), "+dbpostgres.WorkspaceLabelGroupColumns.ID),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list label groups: %w", err)
	}

	groups := make([]entity.LabelGroup, 0, len(models))

	for _, model := range models {
		group, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	return groups, nil
}

func (r *labelGroupRepository) UpdateName(
	ctx context.Context,
	id uuid.UUID,
	name string,
) (entity.LabelGroup, error) {
	updated, err := dbpostgres.WorkspaceLabelGroups(
		dbpostgres.WorkspaceLabelGroupWhere.ID.EQ(id.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceLabelGroupColumns.Name:      name,
		dbpostgres.WorkspaceLabelGroupColumns.UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return entity.LabelGroup{}, translated
		}

		return entity.LabelGroup{}, fmt.Errorf("update label group: %w", err)
	}

	if updated == 0 {
		return entity.LabelGroup{}, entity.ErrLabelGroupNotFound
	}

	model, err := dbpostgres.FindWorkspaceLabelGroup(ctx, r.db.Querier(ctx), id.String())
	if err != nil {
		return entity.LabelGroup{}, fmt.Errorf("read updated label group: %w", err)
	}

	return toEntity(model)
}

func (r *labelGroupRepository) Ungroup(ctx context.Context, groupID uuid.UUID) error {
	querier := r.db.Querier(ctx)

	if _, err := querier.ExecContext(ctx, ungroupApplicationsQuery, groupID.String()); err != nil {
		return fmt.Errorf("ungroup label applications: %w", err)
	}

	if _, err := querier.ExecContext(ctx, ungroupQuery, groupID.String(), time.Now().UTC()); err != nil {
		return fmt.Errorf("ungroup labels: %w", err)
	}

	return nil
}

func (r *labelGroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	deleted, err := dbpostgres.WorkspaceLabelGroups(
		dbpostgres.WorkspaceLabelGroupWhere.ID.EQ(id.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		if translated := translateWriteError(err); !errors.Is(translated, err) {
			return translated
		}

		return fmt.Errorf("delete label group: %w", err)
	}

	if deleted == 0 {
		return entity.ErrLabelGroupNotFound
	}

	return nil
}
