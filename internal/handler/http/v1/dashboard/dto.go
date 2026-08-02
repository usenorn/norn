package dashboard

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
	api "github.com/usenorn/norn/pkg/http/v1/dashboard"
)

func accountDTO(account entity.Account, avatarURL string) api.Account {
	dto := api.Account{
		Id:          account.ID,
		Status:      api.AccountStatus(account.Status),
		Email:       account.Email,
		DisplayName: account.DisplayName,
		Timezone:    account.Timezone,
		HasPassword: account.HasPassword(),
		CreatedAt:   account.CreatedAt,
	}

	if avatarURL != "" {
		dto.AvatarUrl = &avatarURL
	}

	return dto
}

func emailChangeDTO(change entity.EmailChange) api.EmailChange {
	return api.EmailChange{
		Id:          change.ID,
		NewEmail:    change.NewEmail,
		RequestedAt: change.RequestedAt,
		ExpiresAt:   change.ExpiresAt,
	}
}

func workspaceDTO(workspace entity.Workspace) api.Workspace {
	return api.Workspace{
		Id:        workspace.ID,
		Slug:      workspace.Slug,
		Name:      workspace.Name,
		CreatedAt: workspace.CreatedAt,
	}
}

func workspaceDTOs(workspaces []entity.Workspace) []api.Workspace {
	dtos := make([]api.Workspace, len(workspaces))
	for i, workspace := range workspaces {
		dtos[i] = workspaceDTO(workspace)
	}

	return dtos
}

func membershipDTO(membership entity.Membership) api.Membership {
	return api.Membership{
		WorkspaceId: membership.WorkspaceID,
		AccountId:   membership.AccountID,
		Role:        api.MembershipRole(membership.Role),
	}
}

func sessionDTO(session entity.Session, currentID uuid.UUID) api.Session {
	client := api.SessionClient{
		UserAgent: &session.Client.UserAgent,
		Location: &api.SessionLocation{
			CountryCode: &session.Client.Location.CountryCode,
			City:        &session.Client.Location.City,
		},
	}

	if session.Client.IP.IsValid() {
		ip := session.Client.IP.String()
		client.Ip = &ip
	}

	return api.Session{
		Id:         session.ID,
		AuthMethod: api.SessionAuthMethod(session.AuthMethod),
		Client:     client,
		IssuedAt:   session.IssuedAt,
		LastUsedAt: session.LastUsedAt,
		ExpiresAt:  session.ExpiresAt(),
		Current:    session.ID == currentID,
	}
}

func sessionDTOs(sessions []entity.Session, currentID uuid.UUID) []api.Session {
	dtos := make([]api.Session, len(sessions))
	for i, session := range sessions {
		dtos[i] = sessionDTO(session, currentID)
	}

	return dtos
}

func workspaceAuthPolicyDTO(policy entity.WorkspaceAuthPolicy) api.WorkspaceAuthPolicy {
	return api.WorkspaceAuthPolicy{
		WorkspaceId: policy.WorkspaceID,
		Enforcement: api.AuthEnforcement(policy.Enforcement),
	}
}

func membershipDTOs(memberships []entity.Membership) []api.Membership {
	dtos := make([]api.Membership, len(memberships))
	for i, membership := range memberships {
		dtos[i] = membershipDTO(membership)
	}

	return dtos
}
