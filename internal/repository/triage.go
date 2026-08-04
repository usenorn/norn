package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/usenorn/norn/internal/entity"
)

//go:generate go tool mockgen -source=triage.go -destination=triage/mock_triage.go -package=triage -mock_names=Triage=MockTriage

type Triage interface {
	Settings(ctx context.Context, workspaceID, teamID uuid.UUID) (entity.TriageSettings, error)
	Upsert(ctx context.Context, settings entity.TriageSettings) (entity.TriageSettings, error)
	Disable(ctx context.Context, workspaceID, teamID uuid.UUID) error
	Decide(ctx context.Context, issueID uuid.UUID, state entity.TriageState, decidedBy uuid.UUID, at time.Time) error
}
