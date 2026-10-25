package service

import (
	"context"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source=memory_evidence_checks.go -destination=memoryevidence/mock_memory_evidence_checks.go -package=memoryevidence -mock_names=MemoryEvidenceChecks=MockMemoryEvidenceChecks

type MemoryEvidenceChecks interface {
	Schedule(ctx context.Context, workspaceID uuid.UUID)
	Check(ctx context.Context, workspaceID uuid.UUID) error
	Sweep(ctx context.Context) error
}
