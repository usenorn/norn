package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=codebases.go -destination=codebase/mock_codebases.go -package=codebase -mock_names=Codebases=MockCodebases

type ConnectCodebaseInput struct {
	Name           string
	RootPath       string
	Repositories   []entity.CodebaseRepository
	SharedFiles    []string
	Runtimes       []entity.CodebaseRuntime
	Tools          []entity.CodingTool
	PreviewGateway entity.GatewayReach
}

type Codebases interface {
	Connect(ctx context.Context, input ConnectCodebaseInput) (entity.Codebase, error)
	Mine(ctx context.Context) ([]entity.Codebase, error)
	Confirm(ctx context.Context, codebaseID uuid.UUID) (entity.Codebase, error)
	Disconnect(ctx context.Context, codebaseID uuid.UUID) (entity.Codebase, error)
	ListByAgent(ctx context.Context, workspaceID, agentID uuid.UUID) ([]entity.Codebase, error)
	DisconnectAgentCodebase(ctx context.Context, workspaceID, agentID, codebaseID uuid.UUID) (entity.Codebase, error)
}
