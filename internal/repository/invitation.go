package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=invitation.go -destination=invitation/mock_invitation.go -package=invitation -mock_names=Invitation=MockInvitation

type Invitation interface {
	Create(ctx context.Context, invitation entity.Invitation) (entity.Invitation, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.Invitation, error)
	GetByTokenHash(ctx context.Context, tokenHash []byte) (entity.Invitation, error)
	ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID, status entity.InvitationStatus) ([]entity.Invitation, error)
	RevokePendingByEmail(ctx context.Context, workspaceID uuid.UUID, email string, revokedAt time.Time) error
	Refresh(ctx context.Context, id uuid.UUID, tokenHash []byte, expiresAt time.Time, delivery entity.InvitationDelivery) (entity.Invitation, error)
	MarkAccepted(ctx context.Context, id, accountID uuid.UUID, acceptedAt time.Time) error
	MarkRevoked(ctx context.Context, id uuid.UUID, revokedAt time.Time) error
	SetDelivery(ctx context.Context, id uuid.UUID, delivery entity.InvitationDelivery) error
}
