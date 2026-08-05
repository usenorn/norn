package service

import (
	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

type RegisterAgentInput struct {
	WorkspaceID    uuid.UUID
	Name           string
	OwnerAccountID uuid.UUID
	Scopes         entity.APIScopeSet
	AllTeams       bool
	TeamIDs        []uuid.UUID
	ActionLimit    *int
}

type RegisteredAgent struct {
	Agent entity.Agent
	Token entity.APIToken
	Value string
}

type OwnedAgent struct {
	Agent      entity.Agent
	OwnerName  string
	OwnerEmail string
}

type ConfigureAgentInput struct {
	WorkspaceID      uuid.UUID
	TeamID           uuid.UUID
	HoldComments     bool
	HoldStateChanges bool
	HoldIssueEdits   bool
}
