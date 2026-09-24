package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=agent_capabilities.go -destination=agentcapability/mock_agent_capabilities.go -package=agentcapability -mock_names=AgentCapabilities=MockAgentCapabilities

type AgentCapabilities interface {
	ListForAgent(ctx context.Context, workspaceID, agentID uuid.UUID) (AgentCapabilitySet, error)
	ListLibrary(ctx context.Context, workspaceID uuid.UUID) (LibraryCapabilities, error)

	ResolveSkillSource(ctx context.Context, workspaceID uuid.UUID, source string) (entity.AgentSkillDiscovery, error)
	ImportSkill(ctx context.Context, owner CapabilityOwner, input ImportSkillInput) (entity.AgentSkill, error)
	WriteSkill(ctx context.Context, owner CapabilityOwner, draft SkillDraft) (entity.AgentSkill, error)
	RewriteSkill(ctx context.Context, workspaceID, skillID uuid.UUID, draft SkillDraft) (entity.AgentSkill, error)
	PullSkill(ctx context.Context, workspaceID, skillID uuid.UUID) (entity.AgentSkill, error)
	DeleteSkill(ctx context.Context, workspaceID, skillID uuid.UUID) error

	SearchRegistry(ctx context.Context, workspaceID uuid.UUID, query string) ([]entity.AgentMCPRegistryEntry, error)
	CreateMCPServer(ctx context.Context, owner CapabilityOwner, input entity.AgentMCPServerInput) (AgentMCPServerView, error)
	UpdateMCPServer(ctx context.Context, workspaceID, serverID uuid.UUID, input entity.AgentMCPServerInput) (AgentMCPServerView, error)
	DeleteMCPServer(ctx context.Context, workspaceID, serverID uuid.UUID) error

	AttachSkill(ctx context.Context, workspaceID, agentID, skillID uuid.UUID) error
	DetachSkill(ctx context.Context, workspaceID, agentID, skillID uuid.UUID) error
	AttachMCPServer(ctx context.Context, workspaceID, agentID, serverID uuid.UUID) error
	DetachMCPServer(ctx context.Context, workspaceID, agentID, serverID uuid.UUID) error

	BeginMCPConnect(ctx context.Context, input BeginMCPConnectInput) (string, error)
	CompleteMCPConnect(ctx context.Context, state, code, redirectURI string) (CompletedMCPConnect, error)
	DisconnectMCP(ctx context.Context, workspaceID, serverID uuid.UUID) error
}
