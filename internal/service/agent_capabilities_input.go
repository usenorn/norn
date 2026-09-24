package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type CapabilityOwner struct {
	WorkspaceID uuid.UUID
	AgentID     *uuid.UUID
}

type ImportSkillInput struct {
	Source string
	Path   *string
}

type SkillDraft struct {
	Instructions string
	Archive      []byte
}

type AgentMCPServerView struct {
	Server     entity.AgentMCPServer
	Connection *entity.AgentMCPConnection
}

type AgentCapabilitySet struct {
	Skills     []entity.AgentSkill
	MCPServers []AgentMCPServerView
}

type LibrarySkill struct {
	Skill    entity.AgentSkill
	AgentIDs []uuid.UUID
}

type LibraryMCPServer struct {
	View     AgentMCPServerView
	AgentIDs []uuid.UUID
}

type LibraryCapabilities struct {
	Skills     []LibrarySkill
	MCPServers []LibraryMCPServer
}

type BeginMCPConnectInput struct {
	WorkspaceID uuid.UUID
	ServerID    uuid.UUID
	ReturnTo    string
	RedirectURI string
}

type CompletedMCPConnect struct {
	WorkspaceID uuid.UUID
	ServerID    uuid.UUID
	ReturnTo    string
}
