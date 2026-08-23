package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=runner.go -destination=runner/mock_runner.go -package=runner -mock_names=Runner=MockRunner

type Runner interface {
	Enrol(ctx context.Context, runner entity.Runner) (entity.Runner, error)
	GetByID(ctx context.Context, runnerID uuid.UUID) (entity.Runner, error)
	GetByRefreshHash(ctx context.Context, refreshHash []byte) (entity.Runner, error)
	ListByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]entity.Runner, error)
	ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]entity.Runner, error)
	Revoke(ctx context.Context, workspaceID, runnerID uuid.UUID, revokedAt time.Time) error
	RecordSeen(ctx context.Context, runnerID uuid.UUID, seenAt time.Time) error
	SetPaused(
		ctx context.Context, workspaceID, runnerID uuid.UUID, pausedAt *time.Time,
	) (entity.Runner, error)
	RecordVersion(ctx context.Context, runnerID uuid.UUID, version string) error
}
