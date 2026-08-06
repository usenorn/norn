package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=cycle.go -destination=cycle/mock_cycle.go -package=cycle -mock_names=Cycle=MockCycle,CycleCadence=MockCycleCadence,CycleScopeChange=MockCycleScopeChange

type CycleFilter struct {
	TeamID *uuid.UUID
}

type Cycle interface {
	Create(ctx context.Context, cycle entity.Cycle) (entity.Cycle, error)
	GetVisible(ctx context.Context, workspaceID, cycleID uuid.UUID, scope entity.TeamScope) (entity.Cycle, error)
	GetByNumber(ctx context.Context, workspaceID, teamID uuid.UUID, number int, scope entity.TeamScope) (entity.Cycle, error)
	ListVisible(ctx context.Context, scope entity.TeamScope, filter CycleFilter) ([]entity.Cycle, error)
	ListByTeamID(ctx context.Context, teamID uuid.UUID) ([]entity.Cycle, error)
	LockByID(ctx context.Context, cycleID uuid.UUID) (entity.Cycle, error)
	Close(ctx context.Context, cycleID uuid.UUID, closedAt time.Time, closedBy *uuid.UUID, rollover entity.CycleRollover) (entity.Cycle, error)
	NextAfter(ctx context.Context, teamID uuid.UUID, endsOn string) (entity.Cycle, error)
	HighestNumber(ctx context.Context, teamID uuid.UUID) (int, error)
	Delete(ctx context.Context, cycleID uuid.UUID) error
	DeleteVacantFrom(ctx context.Context, teamID uuid.UUID, from string) error
}

type CadenceListing struct {
	Cadence  entity.CycleCadence
	Timezone string
}

type CycleCadence interface {
	Get(ctx context.Context, teamID uuid.UUID) (entity.CycleCadence, error)
	Lock(ctx context.Context, teamID uuid.UUID) (entity.CycleCadence, error)
	Upsert(ctx context.Context, cadence entity.CycleCadence) (entity.CycleCadence, error)
	Delete(ctx context.Context, teamID uuid.UUID) error
	ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]entity.CycleCadence, error)
	ListAll(ctx context.Context) ([]CadenceListing, error)
}

type CycleScopeChange interface {
	Record(ctx context.Context, change entity.CycleScopeChange) error
	ListByCycleID(ctx context.Context, cycleID uuid.UUID) ([]entity.CycleScopeChange, error)
}
