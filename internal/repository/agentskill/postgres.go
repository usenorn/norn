package agentskill

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
	uniqueViolationCode = "23505"
	agentNameIndex      = "workspace_agent_skills_agent_name_key"
	libraryNameIndex    = "workspace_agent_skills_library_name_key"
	attachmentKey       = "workspace_agent_skill_attachments_pkey"
	orderByName         = "lower(name)"
)

type skillRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.AgentSkill {
	return &skillRepository{db: db}
}

func toEntity(model *dbpostgres.WorkspaceAgentSkill) (entity.AgentSkill, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.AgentSkill{}, fmt.Errorf("parse skill id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.AgentSkill{}, fmt.Errorf("parse skill workspace id: %w", err)
	}

	createdBy, err := uuid.Parse(model.CreatedByAccountID)
	if err != nil {
		return entity.AgentSkill{}, fmt.Errorf("parse skill author id: %w", err)
	}

	skill := entity.AgentSkill{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        model.Name,
		Description: model.Description,
		Source:      entity.AgentSkillSource(model.Source),
		Origin: entity.AgentSkillOrigin{
			Repository: model.Origin,
			Path:       model.Path,
			Ref:        model.Ref,
			Revision:   model.Revision,
		},
		Instructions: model.Instructions,
		ContentHash:  model.ContentHash,
		ObjectKey:    model.ObjectKey,
		SizeBytes:    model.SizeBytes,
		FileCount:    model.FileCount,
		CreatedBy:    createdBy,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}

	if model.AgentID.Valid {
		agentID, err := uuid.Parse(model.AgentID.String)
		if err != nil {
			return entity.AgentSkill{}, fmt.Errorf("parse skill agent id: %w", err)
		}

		skill.AgentID = &agentID
	}

	return skill, nil
}

func toEntities(models dbpostgres.WorkspaceAgentSkillSlice) ([]entity.AgentSkill, error) {
	skills := make([]entity.AgentSkill, 0, len(models))

	for _, model := range models {
		skill, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		skills = append(skills, skill)
	}

	return skills, nil
}

func agentColumn(agentID *uuid.UUID) null.String {
	if agentID == nil {
		return null.String{}
	}

	return null.StringFrom(agentID.String())
}

func nameTaken(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolationCode &&
		(pgErr.ConstraintName == agentNameIndex || pgErr.ConstraintName == libraryNameIndex)
}

func (r *skillRepository) Create(ctx context.Context, skill entity.AgentSkill) (entity.AgentSkill, error) {
	now := time.Now().UTC()

	model := &dbpostgres.WorkspaceAgentSkill{
		ID:                 skill.ID.String(),
		WorkspaceID:        skill.WorkspaceID.String(),
		AgentID:            agentColumn(skill.AgentID),
		Name:               skill.Name,
		Description:        skill.Description,
		Source:             string(skill.Source),
		Origin:             skill.Origin.Repository,
		Path:               skill.Origin.Path,
		Ref:                skill.Origin.Ref,
		Revision:           skill.Origin.Revision,
		Instructions:       skill.Instructions,
		ContentHash:        skill.ContentHash,
		ObjectKey:          skill.ObjectKey,
		SizeBytes:          skill.SizeBytes,
		FileCount:          skill.FileCount,
		CreatedByAccountID: skill.CreatedBy.String(),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		if nameTaken(err) {
			return entity.AgentSkill{}, entity.ErrAgentSkillNameTaken
		}

		return entity.AgentSkill{}, fmt.Errorf("insert agent skill: %w", err)
	}

	return toEntity(model)
}

func (r *skillRepository) Replace(ctx context.Context, skill entity.AgentSkill) (entity.AgentSkill, error) {
	rows, err := dbpostgres.WorkspaceAgentSkills(
		dbpostgres.WorkspaceAgentSkillWhere.WorkspaceID.EQ(skill.WorkspaceID.String()),
		dbpostgres.WorkspaceAgentSkillWhere.ID.EQ(skill.ID.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceAgentSkillColumns.Name:         skill.Name,
		dbpostgres.WorkspaceAgentSkillColumns.Description:  skill.Description,
		dbpostgres.WorkspaceAgentSkillColumns.Ref:          skill.Origin.Ref,
		dbpostgres.WorkspaceAgentSkillColumns.Revision:     skill.Origin.Revision,
		dbpostgres.WorkspaceAgentSkillColumns.Instructions: skill.Instructions,
		dbpostgres.WorkspaceAgentSkillColumns.ContentHash:  skill.ContentHash,
		dbpostgres.WorkspaceAgentSkillColumns.ObjectKey:    skill.ObjectKey,
		dbpostgres.WorkspaceAgentSkillColumns.SizeBytes:    skill.SizeBytes,
		dbpostgres.WorkspaceAgentSkillColumns.FileCount:    skill.FileCount,
		dbpostgres.WorkspaceAgentSkillColumns.UpdatedAt:    time.Now().UTC(),
	})
	if err != nil {
		if nameTaken(err) {
			return entity.AgentSkill{}, entity.ErrAgentSkillNameTaken
		}

		return entity.AgentSkill{}, fmt.Errorf("replace agent skill: %w", err)
	}

	if rows == 0 {
		return entity.AgentSkill{}, entity.ErrAgentSkillNotFound
	}

	return r.Get(ctx, skill.WorkspaceID, skill.ID)
}

func (r *skillRepository) Get(ctx context.Context, workspaceID, skillID uuid.UUID) (entity.AgentSkill, error) {
	model, err := dbpostgres.WorkspaceAgentSkills(
		dbpostgres.WorkspaceAgentSkillWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentSkillWhere.ID.EQ(skillID.String()),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.AgentSkill{}, entity.ErrAgentSkillNotFound
		}

		return entity.AgentSkill{}, fmt.Errorf("read agent skill: %w", err)
	}

	return toEntity(model)
}

func (r *skillRepository) ListByAgent(
	ctx context.Context,
	workspaceID, agentID uuid.UUID,
) ([]entity.AgentSkill, error) {
	models, err := dbpostgres.WorkspaceAgentSkills(
		dbpostgres.WorkspaceAgentSkillWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.Expr(
			dbpostgres.WorkspaceAgentSkillWhere.AgentID.EQ(null.StringFrom(agentID.String())),
			qm.Or(
				dbpostgres.WorkspaceAgentSkillColumns.ID+
					" IN (SELECT skill_id FROM workspace_agent_skill_attachments WHERE agent_id = ?)",
				agentID.String(),
			),
		),
		qm.OrderBy(orderByName),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list agent skills: %w", err)
	}

	return toEntities(models)
}

func (r *skillRepository) ListLibrary(ctx context.Context, workspaceID uuid.UUID) ([]entity.AgentSkill, error) {
	models, err := dbpostgres.WorkspaceAgentSkills(
		dbpostgres.WorkspaceAgentSkillWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentSkillWhere.AgentID.IsNull(),
		qm.OrderBy(orderByName),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list library skills: %w", err)
	}

	return toEntities(models)
}

func (r *skillRepository) CountOwned(
	ctx context.Context,
	workspaceID uuid.UUID,
	agentID *uuid.UUID,
) (int, error) {
	owner := dbpostgres.WorkspaceAgentSkillWhere.AgentID.IsNull()
	if agentID != nil {
		owner = dbpostgres.WorkspaceAgentSkillWhere.AgentID.EQ(null.StringFrom(agentID.String()))
	}

	count, err := dbpostgres.WorkspaceAgentSkills(
		dbpostgres.WorkspaceAgentSkillWhere.WorkspaceID.EQ(workspaceID.String()),
		owner,
	).Count(ctx, r.db.Querier(ctx))
	if err != nil {
		return 0, fmt.Errorf("count agent skills: %w", err)
	}

	return int(count), nil
}

func (r *skillRepository) Delete(ctx context.Context, workspaceID, skillID uuid.UUID) error {
	rows, err := dbpostgres.WorkspaceAgentSkills(
		dbpostgres.WorkspaceAgentSkillWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentSkillWhere.ID.EQ(skillID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("delete agent skill: %w", err)
	}

	if rows == 0 {
		return entity.ErrAgentSkillNotFound
	}

	return nil
}

func (r *skillRepository) Attach(ctx context.Context, attachment entity.AgentCapabilityAttachment) error {
	model := &dbpostgres.WorkspaceAgentSkillAttachment{
		AgentID:             attachment.AgentID.String(),
		SkillID:             attachment.CapabilityID.String(),
		WorkspaceID:         attachment.WorkspaceID.String(),
		AttachedByAccountID: attachment.AttachedBy.String(),
		AttachedAt:          time.Now().UTC(),
	}

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == attachmentKey {
			return entity.ErrAgentCapabilityAttached
		}

		return fmt.Errorf("attach agent skill: %w", err)
	}

	return nil
}

func (r *skillRepository) Detach(ctx context.Context, workspaceID, agentID, skillID uuid.UUID) error {
	rows, err := dbpostgres.WorkspaceAgentSkillAttachments(
		dbpostgres.WorkspaceAgentSkillAttachmentWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceAgentSkillAttachmentWhere.AgentID.EQ(agentID.String()),
		dbpostgres.WorkspaceAgentSkillAttachmentWhere.SkillID.EQ(skillID.String()),
	).DeleteAll(ctx, r.db.Querier(ctx))
	if err != nil {
		return fmt.Errorf("detach agent skill: %w", err)
	}

	if rows == 0 {
		return entity.ErrAgentCapabilityNotAttached
	}

	return nil
}

func (r *skillRepository) AttachedAgents(
	ctx context.Context,
	workspaceID uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {
	models, err := dbpostgres.WorkspaceAgentSkillAttachments(
		dbpostgres.WorkspaceAgentSkillAttachmentWhere.WorkspaceID.EQ(workspaceID.String()),
		qm.OrderBy(dbpostgres.WorkspaceAgentSkillAttachmentColumns.AttachedAt),
	).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list agent skill attachments: %w", err)
	}

	attached := make(map[uuid.UUID][]uuid.UUID, len(models))

	for _, model := range models {
		skillID, err := uuid.Parse(model.SkillID)
		if err != nil {
			return nil, fmt.Errorf("parse attached skill id: %w", err)
		}

		agentID, err := uuid.Parse(model.AgentID)
		if err != nil {
			return nil, fmt.Errorf("parse attached agent id: %w", err)
		}

		attached[skillID] = append(attached[skillID], agentID)
	}

	return attached, nil
}
