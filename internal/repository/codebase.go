package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=codebase.go -destination=codebase/mock_codebase.go -package=codebase -mock_names=Codebase=MockCodebase

type CodebaseInventory struct {
	Name           string
	RootPath       string
	Repositories   []entity.CodebaseRepository
	SharedFiles    []string
	Runtimes       []entity.CodebaseRuntime
	Tools          []entity.CodingTool
	PreviewGateway entity.GatewayReach
}

type Codebase interface {
	Connect(ctx context.Context, runnerID uuid.UUID, inventory CodebaseInventory, connectedAt time.Time) (entity.Codebase, error)
	GetByID(ctx context.Context, codebaseID uuid.UUID) (entity.Codebase, error)
	GetLiveByRoot(ctx context.Context, runnerID uuid.UUID, rootPath string) (entity.Codebase, error)
	ListByRunnerID(ctx context.Context, runnerID uuid.UUID) ([]entity.Codebase, error)
	ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]entity.Codebase, error)
	Replace(ctx context.Context, codebaseID uuid.UUID, inventory CodebaseInventory, state entity.CodebaseState, at time.Time) (entity.Codebase, error)
	Confirm(ctx context.Context, codebaseID uuid.UUID, at time.Time) (entity.Codebase, error)
	Disconnect(ctx context.Context, codebaseID uuid.UUID, at time.Time) (entity.Codebase, error)
	RecordSeen(ctx context.Context, codebaseID uuid.UUID, seenAt time.Time) error
}
