package invitation

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

	dbpostgres "github.com/usenorn/norn/internal/db/postgres"
	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/pkg/postgres"
	"github.com/usenorn/norn/internal/repository"
)

func toEntity(model *dbpostgres.WorkspaceInvitation) (entity.Invitation, error) {
	id, err := uuid.Parse(model.ID)
	if err != nil {
		return entity.Invitation{}, fmt.Errorf("parse invitation id: %w", err)
	}

	workspaceID, err := uuid.Parse(model.WorkspaceID)
	if err != nil {
		return entity.Invitation{}, fmt.Errorf("parse invitation workspace id: %w", err)
	}

	invitation := entity.Invitation{
		ID:          id,
		WorkspaceID: workspaceID,
		Email:       model.Email,
		Role:        entity.MembershipRole(model.Role),
		Status:      entity.InvitationStatus(model.Status),
		Delivery:    entity.InvitationDelivery(model.Delivery),
		TokenHash:   model.TokenHash,
		InvitedAt:   model.InvitedAt,
		ExpiresAt:   model.ExpiresAt,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}

	if model.InvitedByAccountID.Valid {
		invitedBy, err := uuid.Parse(model.InvitedByAccountID.String)
		if err != nil {
			return entity.Invitation{}, fmt.Errorf("parse invitation inviter id: %w", err)
		}

		invitation.InvitedByAccountID = &invitedBy
	}

	if model.AcceptedByAccountID.Valid {
		acceptedBy, err := uuid.Parse(model.AcceptedByAccountID.String)
		if err != nil {
			return entity.Invitation{}, fmt.Errorf("parse invitation acceptor id: %w", err)
		}

		invitation.AcceptedByAccountID = &acceptedBy
	}

	if model.AcceptedAt.Valid {
		acceptedAt := model.AcceptedAt.Time
		invitation.AcceptedAt = &acceptedAt
	}

	if model.RevokedAt.Valid {
		revokedAt := model.RevokedAt.Time
		invitation.RevokedAt = &revokedAt
	}

	if model.R != nil {
		teamIDs := make([]uuid.UUID, 0, len(model.R.InvitationWorkspaceInvitationTeams))

		for _, link := range model.R.InvitationWorkspaceInvitationTeams {
			teamID, err := uuid.Parse(link.TeamID)
			if err != nil {
				return entity.Invitation{}, fmt.Errorf("parse invitation team id: %w", err)
			}

			teamIDs = append(teamIDs, teamID)
		}

		invitation.TeamIDs = teamIDs
	}

	return invitation, nil
}

func toModel(invitation entity.Invitation) *dbpostgres.WorkspaceInvitation {
	model := &dbpostgres.WorkspaceInvitation{
		ID:          invitation.ID.String(),
		WorkspaceID: invitation.WorkspaceID.String(),
		Email:       invitation.Email,
		Role:        string(invitation.Role),
		Status:      string(invitation.Status),
		Delivery:    string(invitation.Delivery),
		TokenHash:   invitation.TokenHash,
		InvitedAt:   invitation.InvitedAt,
		ExpiresAt:   invitation.ExpiresAt,
		CreatedAt:   invitation.CreatedAt,
		UpdatedAt:   invitation.UpdatedAt,
	}

	if invitation.InvitedByAccountID != nil {
		model.InvitedByAccountID = null.StringFrom(invitation.InvitedByAccountID.String())
	}

	if invitation.AcceptedByAccountID != nil {
		model.AcceptedByAccountID = null.StringFrom(invitation.AcceptedByAccountID.String())
	}

	if invitation.AcceptedAt != nil {
		model.AcceptedAt = null.TimeFrom(*invitation.AcceptedAt)
	}

	if invitation.RevokedAt != nil {
		model.RevokedAt = null.TimeFrom(*invitation.RevokedAt)
	}

	return model
}

func toEntities(models dbpostgres.WorkspaceInvitationSlice) ([]entity.Invitation, error) {
	invitations := make([]entity.Invitation, len(models))

	for i, model := range models {
		invitation, err := toEntity(model)
		if err != nil {
			return nil, err
		}

		invitations[i] = invitation
	}

	return invitations, nil
}

func loadTeams() qm.QueryMod {
	return qm.Load(dbpostgres.WorkspaceInvitationRels.InvitationWorkspaceInvitationTeams)
}

type invitationRepository struct {
	db *postgres.Client
}

func New(db *postgres.Client) repository.Invitation {
	return &invitationRepository{db: db}
}

func (r *invitationRepository) Create(ctx context.Context, invitation entity.Invitation) (entity.Invitation, error) {
	if invitation.ID == uuid.Nil {
		invitation.ID = uuid.New()
	}

	now := time.Now().UTC()
	invitation.CreatedAt = now
	invitation.UpdatedAt = now

	model := toModel(invitation)

	if err := model.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
		return entity.Invitation{}, fmt.Errorf("insert invitation: %w", err)
	}

	for _, teamID := range invitation.TeamIDs {
		link := &dbpostgres.WorkspaceInvitationTeam{
			InvitationID: model.ID,
			TeamID:       teamID.String(),
			CreatedAt:    now,
		}

		if err := link.Insert(ctx, r.db.Querier(ctx), boil.Infer()); err != nil {
			return entity.Invitation{}, fmt.Errorf("insert invitation team: %w", err)
		}
	}

	created, err := toEntity(model)
	if err != nil {
		return entity.Invitation{}, err
	}

	created.TeamIDs = invitation.TeamIDs

	return created, nil
}

func (r *invitationRepository) GetByID(ctx context.Context, id uuid.UUID) (entity.Invitation, error) {
	model, err := dbpostgres.WorkspaceInvitations(
		dbpostgres.WorkspaceInvitationWhere.ID.EQ(id.String()),
		loadTeams(),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Invitation{}, entity.ErrInvitationNotFound
		}

		return entity.Invitation{}, fmt.Errorf("find invitation by id: %w", err)
	}

	return toEntity(model)
}

func (r *invitationRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.Invitation, error) {
	model, err := dbpostgres.WorkspaceInvitations(
		dbpostgres.WorkspaceInvitationWhere.TokenHash.EQ(tokenHash),
		loadTeams(),
	).One(ctx, r.db.Querier(ctx))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Invitation{}, entity.ErrInvitationNotFound
		}

		return entity.Invitation{}, fmt.Errorf("find invitation by token: %w", err)
	}

	return toEntity(model)
}

func (r *invitationRepository) ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, status entity.InvitationStatus) ([]entity.Invitation, error) {
	mods := []qm.QueryMod{
		dbpostgres.WorkspaceInvitationWhere.WorkspaceID.EQ(workspaceID.String()),
		loadTeams(),
		qm.OrderBy(dbpostgres.WorkspaceInvitationColumns.InvitedAt + " DESC"),
	}

	if status != "" {
		mods = append(mods, dbpostgres.WorkspaceInvitationWhere.Status.EQ(string(status)))
	}

	models, err := dbpostgres.WorkspaceInvitations(mods...).All(ctx, r.db.Querier(ctx))
	if err != nil {
		return nil, fmt.Errorf("list workspace invitations: %w", err)
	}

	return toEntities(models)
}

func (r *invitationRepository) RevokePendingByEmail(ctx context.Context, workspaceID uuid.UUID, email string, revokedAt time.Time) error {
	if _, err := dbpostgres.WorkspaceInvitations(
		dbpostgres.WorkspaceInvitationWhere.WorkspaceID.EQ(workspaceID.String()),
		dbpostgres.WorkspaceInvitationWhere.Status.EQ(string(entity.InvitationStatusPending)),
		qm.Where("lower(email) = lower(?)", email),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceInvitationColumns.Status:    string(entity.InvitationStatusRevoked),
		dbpostgres.WorkspaceInvitationColumns.RevokedAt: null.TimeFrom(revokedAt),
		dbpostgres.WorkspaceInvitationColumns.UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("revoke pending invitations: %w", err)
	}

	return nil
}

func (r *invitationRepository) Refresh(
	ctx context.Context,
	id uuid.UUID,
	tokenHash []byte,
	expiresAt time.Time,
	delivery entity.InvitationDelivery,
) (entity.Invitation, error) {
	updated, err := dbpostgres.WorkspaceInvitations(
		dbpostgres.WorkspaceInvitationWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceInvitationWhere.Status.EQ(string(entity.InvitationStatusPending)),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceInvitationColumns.TokenHash: tokenHash,
		dbpostgres.WorkspaceInvitationColumns.ExpiresAt: expiresAt,
		dbpostgres.WorkspaceInvitationColumns.Delivery:  string(delivery),
		dbpostgres.WorkspaceInvitationColumns.InvitedAt: time.Now().UTC(),
		dbpostgres.WorkspaceInvitationColumns.UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return entity.Invitation{}, fmt.Errorf("refresh invitation: %w", err)
	}

	if updated == 0 {
		return entity.Invitation{}, entity.ErrInvitationNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *invitationRepository) MarkAccepted(ctx context.Context, id, accountID uuid.UUID, acceptedAt time.Time) error {
	updated, err := dbpostgres.WorkspaceInvitations(
		dbpostgres.WorkspaceInvitationWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceInvitationWhere.Status.EQ(string(entity.InvitationStatusPending)),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceInvitationColumns.Status:              string(entity.InvitationStatusAccepted),
		dbpostgres.WorkspaceInvitationColumns.AcceptedAt:          null.TimeFrom(acceptedAt),
		dbpostgres.WorkspaceInvitationColumns.AcceptedByAccountID: null.StringFrom(accountID.String()),
		dbpostgres.WorkspaceInvitationColumns.UpdatedAt:           time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("mark invitation accepted: %w", err)
	}

	if updated == 0 {
		return entity.ErrInvitationAccepted
	}

	return nil
}

func (r *invitationRepository) MarkRevoked(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	updated, err := dbpostgres.WorkspaceInvitations(
		dbpostgres.WorkspaceInvitationWhere.ID.EQ(id.String()),
		dbpostgres.WorkspaceInvitationWhere.Status.EQ(string(entity.InvitationStatusPending)),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceInvitationColumns.Status:    string(entity.InvitationStatusRevoked),
		dbpostgres.WorkspaceInvitationColumns.RevokedAt: null.TimeFrom(revokedAt),
		dbpostgres.WorkspaceInvitationColumns.UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("mark invitation revoked: %w", err)
	}

	if updated == 0 {
		return entity.ErrInvitationNotFound
	}

	return nil
}

func (r *invitationRepository) SetDelivery(ctx context.Context, id uuid.UUID, delivery entity.InvitationDelivery) error {
	if _, err := dbpostgres.WorkspaceInvitations(
		dbpostgres.WorkspaceInvitationWhere.ID.EQ(id.String()),
	).UpdateAll(ctx, r.db.Querier(ctx), dbpostgres.M{
		dbpostgres.WorkspaceInvitationColumns.Delivery:  string(delivery),
		dbpostgres.WorkspaceInvitationColumns.UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("set invitation delivery: %w", err)
	}

	return nil
}
