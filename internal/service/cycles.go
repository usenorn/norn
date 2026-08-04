package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=cycles.go -destination=cycle/mock_cycles.go -package=cycle -mock_names=Cycles=MockCycles

type CycleView struct {
	Cycle entity.Cycle
	Phase entity.CyclePhase
}

type TeamCycle struct {
	TeamID uuid.UUID
	View   CycleView
}

type ListCyclesInput struct {
	TeamID *uuid.UUID
	Phase  entity.CyclePhase
}

type SetCadenceInput struct {
	LengthWeeks int
	StartsOn    time.Weekday
}

type CadenceView struct {
	Cadence  entity.CycleCadence
	Upcoming []CycleView
}

type RolloverOverride struct {
	IssueID     uuid.UUID
	Destination entity.CycleRollover
}

type CloseCycleInput struct {
	Rollover  entity.CycleRollover
	Overrides []RolloverOverride
}

type CycleScope struct {
	Original []entity.Issue
	Added    []entity.Issue
	Changes  []entity.CycleScopeChange
}

type Cycles interface {
	Cadence(ctx context.Context, workspaceID, teamID uuid.UUID) (CadenceView, error)
	SetCadence(ctx context.Context, workspaceID, teamID uuid.UUID, input SetCadenceInput) (CadenceView, error)
	DisableCadence(ctx context.Context, workspaceID, teamID uuid.UUID) error
	List(ctx context.Context, workspaceID uuid.UUID, input ListCyclesInput) ([]CycleView, error)
	Get(ctx context.Context, workspaceID, cycleID uuid.UUID) (CycleView, error)
	Current(ctx context.Context, workspaceID uuid.UUID) ([]TeamCycle, error)
	Scope(ctx context.Context, workspaceID, cycleID uuid.UUID) (CycleScope, error)
	Close(ctx context.Context, workspaceID, cycleID uuid.UUID, input CloseCycleInput) (CycleView, error)
	Generate(ctx context.Context) error
}
