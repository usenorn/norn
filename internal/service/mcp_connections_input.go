package service

import (
	"github.com/usenorn/norn/internal/entity"
)

type RegisterMCPClientInput struct {
	Name         string
	RedirectURIs []string
}

type BeginMCPAuthorizationInput struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
}

type MCPAuthorizationView struct {
	ClientName string
	Capability entity.MCPCapability
	Workspaces []entity.Workspace
}

type MCPAuthorizationDecision struct {
	RedirectTo string
}

type ExchangeMCPCodeInput struct {
	ClientID     string
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type RefreshMCPTokenInput struct {
	ClientID     string
	RefreshToken string
}

type MCPTokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	Capability   entity.MCPCapability
}

type NarrowMCPConnectionInput struct {
	Capability *entity.MCPCapability
	Grants     *entity.APITokenGrants
}

type OwnedMCPConnection struct {
	Connection entity.MCPConnection
	OwnerName  string
	OwnerEmail string
}
