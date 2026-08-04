package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=invitations.go -destination=invitation/mock_invitations.go -package=invitation -mock_names=Invitations=MockInvitations

type Invitations interface {
	Create(ctx context.Context, input CreateInvitationsInput) ([]InvitationResult, error)
	List(ctx context.Context, workspaceID uuid.UUID, status entity.InvitationStatus) ([]entity.Invitation, error)
	Resend(ctx context.Context, workspaceID, invitationID uuid.UUID) (IssuedInvitation, error)
	Revoke(ctx context.Context, workspaceID, invitationID uuid.UUID) error
	Preview(ctx context.Context, token string) (InvitationPreview, error)
	Accept(ctx context.Context, input AcceptInvitationInput) (AcceptedInvitation, error)
	SendInvitation(ctx context.Context, invitationID uuid.UUID, token string) error
}
