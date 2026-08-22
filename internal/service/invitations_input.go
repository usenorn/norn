package service

import (
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type InvitationRecipient struct {
	Email   string
	Role    entity.MembershipRole
	TeamIDs []uuid.UUID
}

type CreateInvitationsInput struct {
	WorkspaceID uuid.UUID
	Recipients  []InvitationRecipient
}

type InvitationResult struct {
	Email      string
	Outcome    entity.InvitationOutcome
	Invitation entity.Invitation
}

type IssuedInvitation struct {
	Invitation entity.Invitation
}

type InvitationPreview struct {
	Workspace     entity.Workspace
	Email         string
	Role          entity.MembershipRole
	InvitedBy     *entity.Account
	InvitedAt     time.Time
	ExpiresAt     time.Time
	Teams         []string
	AccountExists bool
	SSOEnforced   bool
}

type AcceptInvitationInput struct {
	Token       string
	DisplayName string
	Timezone    string
	Password    string
	Client      entity.SessionClient
}

type AcceptedInvitation struct {
	Workspace  entity.Workspace
	Membership entity.Membership
	Session    IssuedSession
	SignedIn   bool
}
